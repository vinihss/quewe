package task

import (
	"fmt"
	"regexp"
	"time"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

const (
	MinPriority = 0
	MaxPriority = 9
)

type ValidatestionError struct{ Msg string }

func (e ValidatestionError) Error() string { return e.Msg }

var (
	ErrInvalidName       = ValidatestionError{Msg: "task: name must match ^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$"}
	ErrInvalidPriority   = ValidatestionError{Msg: "task: priority must be between 0 and 9"}
	ErrInvalidTimeout    = ValidatestionError{Msg: "task: timeout_seconds must be > 0"}
	ErrInvalidMaxRetries = ValidatestionError{Msg: "task: max_retries must be >= 0"}
	ErrInvalidSchedule   = ValidatestionError{Msg: "task: invalid schedule"}
	ErrMissingCapability = ValidatestionError{Msg: "task: target capability is required"}
)

func InvalidSchedule(msg string) ValidatestionError {
	return ValidatestionError{Msg: fmt.Sprintf("task: %s", msg)}
}

type ScheduleType string

const (
	ScheduleManual ScheduleType = "manual"
	ScheduleOnce   ScheduleType = "once"
	ScheduleCron   ScheduleType = "cron"
)

type MisfirePolicy string

const (
	MisfireRunOnce MisfirePolicy = "run_once"
	MisfireSkip    MisfirePolicy = "skip"
	MisfireCatchUp MisfirePolicy = "catch_up"
)

type Schedule struct {
	Type          ScheduleType
	Expression    string
	Timezone      string
	RunAt         *time.Time
	NextRunAt     *time.Time
	LastRunAt     *time.Time
	MisfirePolicy MisfirePolicy
}

func (s Schedule) Validateste() error {
	switch s.Type {
	case ScheduleManual:
		return nil
	case ScheduleOnce:
		if s.RunAt == nil || s.RunAt.IsZero() {
			return InvalidSchedule("once schedule requires run_at")
		}
		return nil
	case ScheduleCron:
		if s.Expression == "" {
			return InvalidSchedule("cron schedule requires expression")
		}
		if s.Timezone == "" {
			return InvalidSchedule("cron schedule requires timezone")
		}
		return nil
	default:
		return InvalidSchedule(fmt.Sprintf("unsupported schedule type %q", s.Type))
	}
}

type Target struct {
	Capability string
	WorkerID   *string
}

func (t Target) Validateste() error {
	if t.Capability == "" {
		return ErrMissingCapability
	}
	return nil
}

type ExecutionPolicy struct {
	TimeoutSeconds int
	MaxRetries     int
	Priority       int
}

func (p ExecutionPolicy) Validateste() error {
	if p.Priority < MinPriority || p.Priority > MaxPriority {
		return ErrInvalidPriority
	}
	if p.TimeoutSeconds <= 0 {
		return ErrInvalidTimeout
	}
	if p.MaxRetries < 0 {
		return ErrInvalidMaxRetries
	}
	return nil
}

func (p ExecutionPolicy) MaxAttempts() int { return p.MaxRetries + 1 }

type Definition struct {
	ID              string
	OwnerID         string
	Name            string
	Type            string
	Description     string
	Payload         []byte
	Target          Target
	ExecutionPolicy ExecutionPolicy
	Schedule        Schedule
	Enabled         bool
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func ValidatesteName(name string) error {
	if !namePattern.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}
