package dto

import (
	"encoding/json"
	"time"
)

// CreateTaskRequest is the transport shape for POST /api/v1/tasks.
// Owner is derived from the authenticated principal, never from the body.
type CreateTaskRequest struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	Description     string                 `json:"description,omitempty"`
	Payload         json.RawMessage        `json:"payload,omitempty"`
	Target          TargetRequest          `json:"target"`
	ExecutionPolicy ExecutionPolicyRequest `json:"execution_policy"`
	Schedule        ScheduleRequest        `json:"schedule"`
	Enabled         *bool                  `json:"enabled,omitempty"`
}

type TargetRequest struct {
	Capability string  `json:"capability"`
	WorkerID   *string `json:"worker_id,omitempty"`
}

type ExecutionPolicyRequest struct {
	TimeoutSeconds int `json:"timeout_seconds"`
	MaxRetries     int `json:"max_retries"`
	Priority       int `json:"priority"`
}

type ScheduleRequest struct {
	Type          string     `json:"type"`
	Expression    string     `json:"expression,omitempty"`
	Timezone      string     `json:"timezone,omitempty"`
	RunAt         *time.Time `json:"run_at,omitempty"`
	MisfirePolicy string     `json:"misfire_policy,omitempty"`
}

type TaskResponse struct {
	ID              string                  `json:"id"`
	OwnerID         string                  `json:"owner_id"`
	Name            string                  `json:"name"`
	Type            string                  `json:"type"`
	Description     string                  `json:"description,omitempty"`
	Payload         json.RawMessage         `json:"payload,omitempty"`
	Target          TargetResponse          `json:"target"`
	ExecutionPolicy ExecutionPolicyResponse `json:"execution_policy"`
	Schedule        ScheduleResponse        `json:"schedule"`
	Enabled         bool                    `json:"enabled"`
	DeletedAt       *time.Time              `json:"deleted_at,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type TargetResponse struct {
	Capability string  `json:"capability"`
	WorkerID   *string `json:"worker_id,omitempty"`
}

type ExecutionPolicyResponse struct {
	TimeoutSeconds int `json:"timeout_seconds"`
	MaxRetries     int `json:"max_retries"`
	Priority       int `json:"priority"`
}

type ScheduleResponse struct {
	Type          string     `json:"type"`
	Expression    string     `json:"expression,omitempty"`
	Timezone      string     `json:"timezone,omitempty"`
	RunAt         *time.Time `json:"run_at,omitempty"`
	NextRunAt     *time.Time `json:"next_run_at,omitempty"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	MisfirePolicy string     `json:"misfire_policy,omitempty"`
}

type ListTasksResponse struct {
	Items      []TaskResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}
