package worker

import (
	"errors"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDraining Status = "draining"
	StatusOffline  Status = "offline"
)

func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusDraining, StatusOffline:
		return true
	default:
		return false
	}
}

type Capability string

type Worker struct {
	WorkerID        string
	Hostname        string
	Version         string
	Status          Status
	Concurrency     int
	LastHeartbeatAt time.Time
	RegisteredAt    time.Time
	Metadata        map[string]any
	Capabilities    []string
}

var (
	ErrMissingWorkerID    = errors.New("worker: worker_id is required")
	ErrInvalidConcurrency = errors.New("worker: concurrency must be > 0")
	ErrInvalidStatus      = errors.New("worker: invalid status")
)

func (w Worker) Validateste() error {
	if w.WorkerID == "" {
		return ErrMissingWorkerID
	}
	if w.Concurrency <= 0 {
		return ErrInvalidConcurrency
	}
	if !w.Status.Valid() {
		return ErrInvalidStatus
	}
	return nil
}
