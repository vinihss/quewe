package taskgroup

import "time"

type Policy string

const (
	PolicyAllCompleted   Policy = "all_completed"
	PolicyAllSettled     Policy = "all_settled"
	PolicyMinimumSuccess Policy = "minimum_success"
	PolicyFirstSuccess   Policy = "first_success"
)

func (p Policy) Valid() bool {
	switch p {
	case PolicyAllCompleted, PolicyAllSettled, PolicyMinimumSuccess, PolicyFirstSuccess:
		return true
	default:
		return false
	}
}

type Status string

const (
	StatusOpen             Status = "open"
	StatusCompleted        Status = "completed"
	StatusCompletedPartial Status = "completed_partial"
	StatusFailed           Status = "failed"
	StatusCancelled        Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusCompleted, StatusCompletedPartial, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

type MemberStatus string

const (
	MemberStatusPending   MemberStatus = "pending"
	MemberStatusRunning   MemberStatus = "running"
	MemberStatusSuccess   MemberStatus = "success"
	MemberStatusFailed    MemberStatus = "failed"
	MemberStatusCancelled MemberStatus = "cancelled"
)

func (m MemberStatus) Valid() bool {
	switch m {
	case MemberStatusPending, MemberStatusRunning, MemberStatusSuccess, MemberStatusFailed, MemberStatusCancelled:
		return true
	default:
		return false
	}
}

type Group struct {
	ID                string
	ParentExecutionID string
	Policy            Policy
	MinSuccess        int
	Deadline          *time.Time
	Status            Status
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (g Group) Validateste() error {
	if g.ID == "" {
		return ErrMissingGroupID
	}
	if !g.Policy.Valid() {
		return ErrInvalidPolicy
	}
	if !g.Status.Valid() {
		return ErrInvalidStatus
	}
	if g.Policy == PolicyMinimumSuccess && g.MinSuccess < 1 {
		return ErrInvalidMinSuccess
	}
	return nil
}

type Member struct {
	GroupID     string
	ExecutionID string
	Required    bool
	JoinedAt    time.Time
}

type GroupStatus struct {
	GroupID   string
	Status    Status
	Total     int
	Completed int
	Failed    int
	Cancelled int
	Pending   int
	Running   int
}

// Evaluate determines the group status based on member statuses.
func Evaluate(policy Policy, members []MemberStatus, required []bool, minSuccess int) Status {
	_ = len(members)
	completed := 0
	failed := 0
	cancelled := 0
	pending := 0
	running := 0

	for i, m := range members {
		switch m {
		case MemberStatusSuccess:
			completed++
		case MemberStatusFailed:
			failed++
		case MemberStatusCancelled:
			cancelled++
		case MemberStatusPending:
			pending++
		case MemberStatusRunning:
			running++
		}
		_ = i
		_ = required
	}

	switch policy {
	case PolicyAllCompleted:
		// all required members must succeed
		allRequiredDone := true
		allRequiredSuccess := true
		for i, m := range members {
			if !required[i] {
				continue
			}
			if !m.IsTerminal() {
				allRequiredDone = false
			}
			if m != MemberStatusSuccess {
				allRequiredSuccess = false
			}
		}
		if !allRequiredDone {
			return StatusOpen
		}
		if allRequiredSuccess {
			return StatusCompleted
		}
		return StatusFailed

	case PolicyAllSettled:
		// wait for all, report partial success
		if pending+running > 0 {
			return StatusOpen
		}
		if failed == 0 && cancelled == 0 {
			return StatusCompleted
		}
		if completed > 0 {
			return StatusCompletedPartial
		}
		return StatusFailed

	case PolicyMinimumSuccess:
		if completed >= minSuccess {
			return StatusCompleted
		}
		if pending+running > 0 {
			return StatusOpen
		}
		if completed > 0 {
			return StatusCompletedPartial
		}
		return StatusFailed

	case PolicyFirstSuccess:
		if completed >= 1 {
			return StatusCompleted
		}
		if pending+running > 0 {
			return StatusOpen
		}
		return StatusFailed

	default:
		return StatusOpen
	}
}

func (m MemberStatus) IsTerminal() bool {
	return m == MemberStatusSuccess || m == MemberStatusFailed || m == MemberStatusCancelled
}

var (
	ErrMissingGroupID    = &GroupError{Code: "missing_group_id", Message: "group: id is required"}
	ErrInvalidPolicy     = &GroupError{Code: "invalid_policy", Message: "group: invalid policy"}
	ErrInvalidStatus     = &GroupError{Code: "invalid_status", Message: "group: invalid status"}
	ErrInvalidMinSuccess = &GroupError{Code: "invalid_min_success", Message: "group: min_success must be >= 1"}
)

type GroupError struct {
	Code    string
	Message string
}

func (e *GroupError) Error() string { return e.Message }
