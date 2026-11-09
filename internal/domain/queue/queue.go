package queue

import (
	"errors"
	"time"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusReserved   Status = "reserved"
	StatusCompleted  Status = "completed"
	StatusDeadLetter Status = "dead_letter"
)

func (s Status) Valid() bool {
	switch s {
	case StatusQueued, StatusReserved, StatusCompleted, StatusDeadLetter:
		return true
	default:
		return false
	}
}

// CanTransition reports whether from->to is allowed for a queue message.
func CanTransition(from, to Status) bool {
	switch from {
	case StatusQueued:
		return to == StatusReserved
	case StatusReserved:
		return to == StatusQueued || to == StatusCompleted || to == StatusDeadLetter
	default:
		return false
	}
}

var (
	ErrInvalidStatus          = errors.New("queue: invalid status")
	ErrInvalidTransition      = errors.New("queue: invalid transition")
	ErrMissingExecutionID     = errors.New("queue: execution_id is required")
	ErrMissingCapability      = errors.New("queue: capability is required")
	ErrInvalidPriority        = errors.New("queue: priority must be 0-9")
	ErrInvalidAttempts        = errors.New("queue: attempts must be >= 0")
	ErrInvalidLeaseGeneration = errors.New("queue: lease_generation must be >= 0")
)

type QueueMessage struct {
	ID              string
	ExecutionID     string
	Queue           string
	Capability      string
	Priority        int
	AvailableAt     time.Time
	Status          Status
	Attempts        int
	ReservedBy      *string
	LeaseExpiresAt  *time.Time
	LeaseGeneration int
	TraceContext    map[string]string
	CreatedAt       time.Time
}

func (m QueueMessage) ValidatesteInvariants() error {
	if m.ExecutionID == "" {
		return ErrMissingExecutionID
	}
	if m.Capability == "" {
		return ErrMissingCapability
	}
	if m.Priority < 0 || m.Priority > 9 {
		return ErrInvalidPriority
	}
	if m.Attempts < 0 {
		return ErrInvalidAttempts
	}
	if m.LeaseGeneration < 0 {
		return ErrInvalidLeaseGeneration
	}
	if !m.Status.Valid() {
		return ErrInvalidStatus
	}
	return nil
}

func (m *QueueMessage) Reserve(workerID string, gen int, leaseExpiresAt time.Time) error {
	if !CanTransition(m.Status, StatusReserved) {
		return ErrInvalidTransition
	}
	if gen < 0 {
		return ErrInvalidLeaseGeneration
	}
	m.Status = StatusReserved
	m.ReservedBy = &workerID
	m.LeaseGeneration = gen
	t := leaseExpiresAt
	m.LeaseExpiresAt = &t
	return nil
}

func (m *QueueMessage) Release() error {
	if !CanTransition(m.Status, StatusQueued) {
		return ErrInvalidTransition
	}
	m.Status = StatusQueued
	m.ReservedBy = nil
	m.LeaseExpiresAt = nil
	return nil
}

func (m *QueueMessage) Complete() error {
	if !CanTransition(m.Status, StatusCompleted) {
		return ErrInvalidTransition
	}
	m.Status = StatusCompleted
	return nil
}

func (m *QueueMessage) DeadLetter() error {
	if !CanTransition(m.Status, StatusDeadLetter) {
		return ErrInvalidTransition
	}
	m.Status = StatusDeadLetter
	return nil
}
