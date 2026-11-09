package execution

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusQueued     Status = "queued"
	StatusRunning    Status = "running"
	StatusCompleted  Status = "completed"
	StatusRetrying   Status = "retrying"
	StatusFailed     Status = "failed"
	StatusDeadLetter Status = "dead_letter"
	StatusCancelled  Status = "cancelled"
)

func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusQueued, StatusRunning, StatusCompleted,
		StatusRetrying, StatusFailed, StatusDeadLetter, StatusCancelled:
		return true
	default:
		return false
	}
}

// CanTransition reports whether from->to is allowed by spec 05.
// Terminal states are immutable except dead_letter -> queued via redrive (not modeled here).
func CanTransition(from, to Status) bool {
	switch from {
	case StatusPending:
		return to == StatusQueued || to == StatusCancelled
	case StatusQueued:
		return to == StatusRunning || to == StatusCancelled
	case StatusRunning:
		switch to {
		case StatusCompleted, StatusRetrying, StatusFailed, StatusDeadLetter, StatusQueued, StatusCancelled:
			return true
		default:
			return false
		}
	case StatusRetrying:
		return to == StatusQueued || to == StatusDeadLetter || to == StatusCancelled
	case StatusDeadLetter:
		return to == StatusQueued // redrive (audited, out of scope F2)
	case StatusCompleted, StatusFailed, StatusCancelled:
		return false
	default:
		return false
	}
}

type Execution struct {
	ID                string
	OwnerID           string
	TaskID            string
	Status            Status
	Attempt           int
	MaxAttempts       int
	ScheduledFor      time.Time
	WorkerID          *string
	StartedAt         *time.Time
	FinishedAt        *time.Time
	CancelRequestedAt *time.Time
	RedriveCount      int
	ResultRef         *string
	ErrorCode         *string
	ErrorMessage      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (e Execution) ValidatesteInvariants() error {
	if e.TaskID == "" {
		return ErrMissingTaskID
	}
	if e.ScheduledFor.IsZero() {
		return ErrMissingScheduledFor
	}
	if e.Attempt < 0 {
		return ErrInvalidAttempt
	}
	if e.MaxAttempts < 1 {
		return ErrInvalidMaxAttempts
	}
	if !e.Status.Valid() {
		return ErrInvalidStatus
	}
	return nil
}
