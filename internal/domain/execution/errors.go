package execution

import "errors"

var (
	ErrMissingTaskID       = errors.New("execution: task_id is required")
	ErrMissingScheduledFor = errors.New("execution: scheduled_for is required")
	ErrInvalidAttempt      = errors.New("execution: attempt must be >= 0")
	ErrInvalidMaxAttempts  = errors.New("execution: max_attempts must be >= 1")
	ErrInvalidStatus       = errors.New("execution: invalid status")
	ErrInvalidTransition   = errors.New("execution: invalid transition")
)
