package dto

import (
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/taskgroup"
)

type CreateGroupRequest struct {
	ParentExecutionID  string          `json:"parent_execution_id"`
	Policy             string          `json:"policy"`
	MinSuccess         int             `json:"min_success"`
	Deadline           *time.Time      `json:"deadline,omitempty"`
	MemberExecutionIDs []string        `json:"member_execution_ids"`
	Required           map[string]bool `json:"required,omitempty"`
}

type GroupResponse struct {
	GroupID string             `json:"group_id"`
	Status  string             `json:"status"`
	Members []taskgroup.Member `json:"members"`
}
