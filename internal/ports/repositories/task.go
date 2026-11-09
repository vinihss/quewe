package repositories

import (
	"context"

	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
)

type TaskListFilter struct {
	OwnerID string
	Enabled *bool
	Type    string
	Limit   int
	Cursor  string
}

type TaskPage struct {
	Items      []task.Definition
	NextCursor string
}

// TaskRepository persists task definitions and schedules (F2 logical aggregate).
// Implementations must enforce owner isolation and (owner_id,name) uniqueness.
type TaskRepository interface {
	Create(ctx context.Context, def task.Definition) error
	GetByID(ctx context.Context, ownerID, id string) (task.Definition, error)
	List(ctx context.Context, filter TaskListFilter) (TaskPage, error)
	GetByTaskID(ctx context.Context, taskID string) (task.Definition, error)
}
