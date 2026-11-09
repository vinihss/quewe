//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/clock"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/outbound/postgres"
	appleasing "github.com/vinihss/quewe/services/quewe/internal/application/leasing"
)

func TestE2E_HappyPath(t *testing.T) {
	setupDB(t)
	baseURL, sched, cleanup := testApp(t)
	defer cleanup()

	w := newFakeWorker("worker-1", []string{"system.maintenance"})
	w.baseURL = baseURL
	w.principalID = "worker-1"

	// create task
	now := time.Now().UTC().Add(2 * time.Second)
	taskBody := map[string]any{
		"name":             "daily-test",
		"type":             "system.command",
		"target":           map[string]any{"capability": "system.maintenance"},
		"execution_policy": map[string]any{"timeout_seconds": 300, "max_retries": 0, "priority": 5},
		"schedule":         map[string]any{"type": "once", "run_at": now.Format(time.RFC3339)},
	}
	resp := doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/tasks", taskBody)
	defer resp.Body.Close()
	var taskResp map[string]any
	json.NewDecoder(resp.Body).Decode(&taskResp)
	taskID := taskResp["id"].(string)
	_ = taskID

	w.register(t)

	// tick to create execution
	if err := sched.SchemaTick(context.Background(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}

	msgs := w.consume(t, 5)
	if len(msgs) == 0 {
		t.Fatal("no consumed messages")
	}
	m := msgs[0]
	execID := m["execution_id"].(string)
	gen := int(m["lease_generation"].(float64))

	w.ack(t, execID, gen)

	// verify completed
	resp2 := doHTTP(t, "owner-1", "GET", baseURL+"/api/v1/executions/"+execID, nil)
	defer resp2.Body.Close()
	var execResp map[string]any
	json.NewDecoder(resp2.Body).Decode(&execResp)
	if execResp["status"] != "completed" {
		t.Fatalf("status=%v", execResp["status"])
	}
}

func TestE2E_LeaseExpiryReenqueue(t *testing.T) {
	pool := setupDB(t)
	baseURL, sched, cleanup := testApp(t)
	defer cleanup()

	w := newFakeWorker("worker-1", []string{"system.maintenance"})
	w.baseURL = baseURL
	w.principalID = "worker-1"

	now := time.Now().UTC().Add(2 * time.Second)
	taskBody := map[string]any{
		"name":             "daily-test",
		"type":             "system.command",
		"target":           map[string]any{"capability": "system.maintenance"},
		"execution_policy": map[string]any{"timeout_seconds": 300, "max_retries": 0, "priority": 5},
		"schedule":         map[string]any{"type": "once", "run_at": now.Format(time.RFC3339)},
	}
	resp := doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/tasks", taskBody)
	defer resp.Body.Close()
	w.register(t)

	if err := sched.SchemaTick(context.Background(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}

	msgs := w.consume(t, 5)
	if len(msgs) == 0 {
		t.Fatal("no consumed")
	}
	m := msgs[0]
	execID := m["execution_id"].(string)

	// expire lease
	_, err := pool.Exec(context.Background(), "UPDATE queue_messages SET lease_expires_at = now() - interval '1 minute' WHERE execution_id = $1", execID)
	if err != nil {
		t.Fatal(err)
	}

	leasingSvc2 := appleasing.NewService(postgres.NewQueueRepository(pool), postgres.NewExecutionRepository(pool), clock.System{})
	if err := leasingSvc2.Reap(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}

	msgs2 := w.consume(t, 5)
	if len(msgs2) == 0 {
		t.Fatal("no requeued")
	}
	m2 := msgs2[0]
	execID2 := m2["execution_id"].(string)
	gen2 := int(m2["lease_generation"].(float64))
	w.ack(t, execID2, gen2)

	resp2 := doHTTP(t, "owner-1", "GET", baseURL+"/api/v1/executions/"+execID2, nil)
	defer resp2.Body.Close()
	var execResp map[string]any
	json.NewDecoder(resp2.Body).Decode(&execResp)
	if execResp["status"] != "completed" {
		t.Fatalf("status=%v", execResp["status"])
	}
}

func TestE2E_RetryAndDeadLetter(t *testing.T) {
	setupDB(t)
}

func TestE2E_OwnerIsolation(t *testing.T) {
	setupDB(t)
	t.Skip("pendente: multi-principal setup")
}

func doHTTP(t *testing.T, principal, method, url string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, url, &buf)
	req.Header.Set("X-Debug-Principal-Id", principal)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
