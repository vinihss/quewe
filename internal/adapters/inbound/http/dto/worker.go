package dto

import "time"

type RegisterWorkerRequest struct {
	WorkerID     string         `json:"worker_id"`
	Hostname     string         `json:"hostname"`
	Version      string         `json:"version"`
	Concurrency  int            `json:"concurrency"`
	Capabilities []string       `json:"capabilities"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type HeartbeatRequest struct {
	Metrics map[string]any `json:"metrics,omitempty"`
}

type HeartbeatResponse struct {
	CancelRequested []string `json:"cancel_requested,omitempty"`
}

type ConsumeRequest struct {
	Capabilities []string `json:"capabilities"`
	Max          int      `json:"max"`
	WaitSeconds  int      `json:"wait_seconds"`
}

type ConsumedMessage struct {
	MessageID       string            `json:"message_id"`
	ExecutionID     string            `json:"execution_id"`
	TaskID          string            `json:"task_id"`
	Type            string            `json:"type"`
	Capability      string            `json:"capability"`
	Attempt         int               `json:"attempt"`
	MaxAttempts     int               `json:"max_attempts"`
	LeaseGeneration int               `json:"lease_generation"`
	LeaseExpiresAt  time.Time         `json:"lease_expires_at"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	IdempotencyKey  string            `json:"idempotency_key"`
	Payload         any               `json:"payload,omitempty"`
	TraceContext    map[string]string `json:"trace_context,omitempty"`
}

type ConsumeResponse struct {
	Messages []ConsumedMessage `json:"messages"`
}

type RenewLeaseRequest struct {
	LeaseGeneration int    `json:"lease_generation"`
	LeaseExpiresAt  string `json:"lease_expires_at"`
}

type ResultRequest struct {
	ContentType string `json:"content_type"`
	InlineBody  any    `json:"inline_body,omitempty"`
	ArtifactURI string `json:"artifact_uri,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}

type AckRequest struct {
	LeaseGeneration int `json:"lease_generation"`
	Result          any `json:"result,omitempty"`
}

type NackRequest struct {
	LeaseGeneration int    `json:"lease_generation"`
	ErrorCode       string `json:"error_code"`
	ErrorMessage    string `json:"error_message"`
	Retryable       bool   `json:"retryable"`
}

type WorkerResponse struct {
	WorkerID        string         `json:"worker_id"`
	Hostname        string         `json:"hostname"`
	Version         string         `json:"version"`
	Status          string         `json:"status"`
	Concurrency     int            `json:"concurrency"`
	LastHeartbeatAt time.Time      `json:"last_heartbeat_at"`
	Capabilities    []string       `json:"capabilities"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type WorkersListResponse struct {
	Workers []WorkerResponse `json:"workers"`
}
