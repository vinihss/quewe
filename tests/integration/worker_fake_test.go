//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

type fakeWorker struct {
	id           string
	capabilities []string
	concurrency  int
	client       *http.Client
	baseURL      string
	principalID  string
}

func newFakeWorker(id string, caps []string) *fakeWorker {
	return &fakeWorker{
		id:           id,
		capabilities: caps,
		concurrency:  1,
		client:       &http.Client{},
		principalID:  id,
	}
}

func (w *fakeWorker) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, w.baseURL+path, &buf)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("X-Debug-Principal-Id", w.principalID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func (w *fakeWorker) register(t *testing.T) {
	t.Helper()
	resp := w.do(t, "POST", "/api/v1/workers/register", map[string]any{
		"hostname":     "test",
		"version":      "1.0",
		"concurrency":  w.concurrency,
		"capabilities": w.capabilities,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status=%d", resp.StatusCode)
	}
}

func (w *fakeWorker) consume(t *testing.T, wait int) []map[string]any {
	t.Helper()
	resp := w.do(t, "POST", "/api/v1/workers/consume", map[string]any{
		"capabilities": w.capabilities,
		"max":          1,
		"wait_seconds": wait,
	})
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("consume status=%d", resp.StatusCode)
	}
	var out struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode consume: %v", err)
	}
	return out.Messages
}

func (w *fakeWorker) ack(t *testing.T, execID string, gen int) {
	t.Helper()
	resp := w.do(t, "POST", fmt.Sprintf("/api/v1/executions/%s/ack", execID), map[string]any{
		"lease_generation": gen,
		"result":           map[string]any{"content_type": "application/json", "inline_body": map[string]any{"status": "ok"}},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("ack status=%d", resp.StatusCode)
	}
}

func (w *fakeWorker) heartbeat(t *testing.T) {
	t.Helper()
	resp := w.do(t, "POST", "/api/v1/workers/heartbeat", map[string]any{})
	defer resp.Body.Close()
}
