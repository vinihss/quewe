package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a thin HTTP client for the task-service API.
// It reuses the same DTOs and error envelope as the server.
type Client struct {
	BaseURL    string
	PrincipalID string
	HTTP       *http.Client
}

// New creates a client with sensible defaults.
func New(baseURL, principalID string) *Client {
	return &Client{
		BaseURL:     baseURL,
		PrincipalID: principalID,
		HTTP:        &http.Client{Timeout: 30 * time.Second},
	}
}

// do executes an HTTP request and decodes the JSON response.
func (c *Client) do(method, path string, body any, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return fmt.Errorf("encode body: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.BaseURL+path, &buf)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-Debug-Principal-Id", c.PrincipalID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errEnv struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &errEnv) == nil && errEnv.Error.Code != "" {
			return fmt.Errorf("%s: %s", errEnv.Error.Code, errEnv.Error.Message)
		}
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// GetTask fetches a task by ID.
func (c *Client) GetTask(id string) (map[string]any, error) {
	var out map[string]any
	err := c.do("GET", "/api/v1/tasks/"+id, nil, &out)
	return out, err
}

// ListTasks lists tasks with optional filters.
func (c *Client) ListTasks(limit int, cursor string) (map[string]any, error) {
	path := "/api/v1/tasks"
	if limit > 0 {
		path += fmt.Sprintf("?limit=%d", limit)
	}
	if cursor != "" {
		sep := "?"
		if limit > 0 {
			sep = "&"
		}
		path += sep + "cursor=" + cursor
	}
	var out map[string]any
	err := c.do("GET", path, nil, &out)
	return out, err
}

// CreateTask creates a new task.
func (c *Client) CreateTask(name, taskType, capability string, scheduleType string, runAt string) (map[string]any, error) {
	body := map[string]any{
		"name":     name,
		"type":     taskType,
		"target":   map[string]any{"capability": capability},
		"execution_policy": map[string]any{
			"timeout_seconds": 300,
			"max_retries":     0,
			"priority":        5,
		},
		"schedule": map[string]any{"type": scheduleType},
	}
	if runAt != "" {
		body["schedule"].(map[string]any)["run_at"] = runAt
	}
	var out map[string]any
	err := c.do("POST", "/api/v1/tasks", body, &out)
	return out, err
}

// GetExecution fetches an execution by ID.
func (c *Client) GetExecution(id string) (map[string]any, error) {
	var out map[string]any
	err := c.do("GET", "/api/v1/executions/"+id, nil, &out)
	return out, err
}

// ListExecutions lists executions.
func (c *Client) ListExecutions() (map[string]any, error) {
	var out map[string]any
	err := c.do("GET", "/api/v1/executions", nil, &out)
	return out, err
}

// RegisterWorker registers a worker.
func (c *Client) RegisterWorker(hostname, version string, concurrency int, caps []string) (map[string]any, error) {
	body := map[string]any{
		"hostname":     hostname,
		"version":      version,
		"concurrency":  concurrency,
		"capabilities": caps,
	}
	var out map[string]any
	err := c.do("POST", "/api/v1/workers/register", body, &out)
	return out, err
}

// ListWorkers lists all workers.
func (c *Client) ListWorkers() (map[string]any, error) {
	var out map[string]any
	err := c.do("GET", "/api/v1/workers", nil, &out)
	return out, err
}

// DrainWorker puts a worker in draining mode.
func (c *Client) DrainWorker(id string) error {
	return c.do("POST", "/api/v1/workers/"+id+"/drain", nil, nil)
}

// GetGroup fetches a group by ID.
func (c *Client) GetGroup(id string) (map[string]any, error) {
	var out map[string]any
	err := c.do("GET", "/api/v1/groups/"+id, nil, &out)
	return out, err
}

// CreateGroup creates a task group.
func (c *Client) CreateGroup(parentExecID, policy string, members []string) (map[string]any, error) {
	body := map[string]any{
		"parent_execution_id":  parentExecID,
		"policy":               policy,
		"member_execution_ids": members,
	}
	var out map[string]any
	err := c.do("POST", "/api/v1/groups", body, &out)
	return out, err
}

// Health checks service health.
func (c *Client) Health() error {
	return c.do("GET", "/healthz", nil, nil)
}