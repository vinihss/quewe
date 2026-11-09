//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestE2E_ResultSuccess(t *testing.T) {
	setupDB(t)
	baseURL, sched, cleanup := testApp(t)
	defer cleanup()

	w := newFakeWorker("worker-1", []string{"system.maintenance"})
	w.baseURL = baseURL
	w.principalID = "worker-1"

	now := time.Now().UTC().Add(2 * time.Second)
	taskBody := map[string]any{
		"name":             "ret-test",
		"type":             "system.command",
		"target":           map[string]any{"capability": "system.maintenance"},
		"execution_policy": map[string]any{"timeout_seconds": 300, "max_retries": 0, "priority": 5},
		"schedule":         map[string]any{"type": "once", "run_at": now.Format(time.RFC3339)},
	}
	resp := doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/tasks", taskBody)
	defer resp.Body.Close()

	var taskResp map[string]any
	json.NewDecoder(resp.Body).Decode(&taskResp)
	execID := taskResp["id"].(string)

	w.register(t)

	if err := sched.SchemaTick(context.Background(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}

	msgs := w.consume(t, 5)
	if len(msgs) == 0 {
		t.Fatal("no messages")
	}
	m := msgs[0]
	gen := int(m["lease_generation"].(float64))

	resp = doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/executions/"+execID+"/result", map[string]any{
		"lease_generation": gen,
		"success":          true,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		t.Fatalf("result success status=%d", resp.StatusCode)
	}

	verified := verifyExecution(t, baseURL, execID)
	if verified["status"] != "completed" {
		t.Fatalf("status=%v expected completed", verified["status"])
	}
}

func TestE2E_ResultFailure(t *testing.T) {
	setupDB(t)
	baseURL, sched, cleanup := testApp(t)
	defer cleanup()

	w := newFakeWorker("worker-1", []string{"system.maintenance"})
	w.baseURL = baseURL
	w.principalID = "worker-1"

	now := time.Now().UTC().Add(2 * time.Second)
	taskBody := map[string]any{
		"name":             "fail-test",
		"type":             "system.command",
		"target":           map[string]any{"capability": "system.maintenance"},
		"execution_policy": map[string]any{"timeout_seconds": 300, "max_retries": 0, "priority": 5},
		"schedule":         map[string]any{"type": "once", "run_at": now.Format(time.RFC3339)},
	}
	resp := doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/tasks", taskBody)
	defer resp.Body.Close()

	var taskResp map[string]any
	json.NewDecoder(resp.Body).Decode(&taskResp)
	execID := taskResp["id"].(string)

	w.register(t)

	if err := sched.SchemaTick(context.Background(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}

	msgs := w.consume(t, 5)
	if len(msgs) == 0 {
		t.Fatal("no messages")
	}
	m := msgs[0]
	gen := int(m["lease_generation"].(float64))

	resp = doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/executions/"+execID+"/result", map[string]any{
		"lease_generation": gen,
		"success":          false,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("result failure status=%d", resp.StatusCode)
	}

	verified := verifyExecution(t, baseURL, execID)
	if verified["status"] != "failed" {
		t.Fatalf("status=%v expected failed", verified["status"])
	}
}

func TestE2E_CancelRunning(t *testing.T) {
	setupDB(t)
	baseURL, sched, cleanup := testApp(t)
	defer cleanup()

	w := newFakeWorker("worker-1", []string{"system.maintenance"})
	w.baseURL = baseURL
	w.principalID = "worker-1"

	now := time.Now().UTC().Add(2 * time.Second)
	taskBody := map[string]any{
		"name":             "cancel-test",
		"type":             "system.command",
		"target":           map[string]any{"capability": "system.maintenance"},
		"execution_policy": map[string]any{"timeout_seconds": 300, "max_retries": 0, "priority": 5},
		"schedule":         map[string]any{"type": "once", "run_at": now.Format(time.RFC3339)},
	}
	resp := doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/tasks", taskBody)
	defer resp.Body.Close()

	var taskResp map[string]any
	json.NewDecoder(resp.Body).Decode(&taskResp)
	execID := taskResp["id"].(string)

	w.register(t)

	if err := sched.SchemaTick(context.Background(), time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}

	msgs := w.consume(t, 5)
	if len(msgs) == 0 {
		t.Fatal("no messages")
	}
	m := msgs[0]
	gen := int(m["lease_generation"].(float64))
	_ = gen

	resp = doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/executions/"+execID+"/cancel", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel status=%d", resp.StatusCode)
	}

	verified := verifyExecution(t, baseURL, execID)
	if verified["status"] != "running" {
		t.Fatalf("status=%v expected running (cancel requested)", verified["status"])
	}
	if verified["cancel_requested_at"] == "" || verified["cancel_requested_at"] == "0001-01-01T00:00:00Z" {
		t.Fatalf("cancel_requested_at not set in DB")
	}
}

func TestE2E_CancelNonExistent(t *testing.T) {
	setupDB(t)
	baseURL, sched, cleanup := testApp(t)
	defer cleanup()
	_ = sched

	resp := doHTTP(t, "owner-1", "POST", baseURL+"/api/v1/executions/nonexistent/cancel", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.StatusCode)
	}
}

func verifyExecution(t *testing.T, baseURL, execID string) map[string]any {
	t.Helper()
	resp := doHTTP(t, "owner-1", "GET", baseURL+"/api/v1/executions/"+execID, nil)
	defer resp.Body.Close()
	var v map[string]any
	json.NewDecoder(resp.Body).Decode(&v)
	return v
}