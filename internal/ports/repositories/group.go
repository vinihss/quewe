package repositories

import (
	"context"

	"github.com/vinihss/quewe/services/quewe/internal/domain/taskgroup"
)

type GroupRepository interface {
	Create(ctx context.Context, g taskgroup.Group, members []taskgroup.Member) error
	Get(ctx context.Context, groupID string) (taskgroup.Group, error)
	ListByParent(ctx context.Context, parentExecutionID string) ([]taskgroup.Group, error)
	UpdateStatus(ctx context.Context, groupID string, status taskgroup.Status) error
	AddMember(ctx context.Context, groupID, executionID string, required bool) error
	GetMembers(ctx context.Context, groupID string) ([]taskgroup.Member, error)
	Evaluate(ctx context.Context, groupID string) (taskgroup.Status, error)
}
