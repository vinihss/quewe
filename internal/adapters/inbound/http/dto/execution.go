package dto

import "time"

type ExecutionResponse struct {
	ID                string     `json:"id"`
	TaskID            string     `json:"task_id"`
	Status            string     `json:"status"`
	Attempt           int        `json:"attempt"`
	MaxAttempts       int        `json:"max_attempts"`
	ScheduledFor      time.Time  `json:"scheduled_for"`
	WorkerID          *string    `json:"worker_id,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	CancelRequestedAt *time.Time `json:"cancel_requested_at,omitempty"`
	RedriveCount      int        `json:"redrive_count"`
	ResultRef         *string    `json:"result_ref,omitempty"`
	ErrorCode         *string    `json:"error_code,omitempty"`
	ErrorMessage      *string    `json:"error_message,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ListExecutionsResponse struct {
	Items      []ExecutionResponse `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}
