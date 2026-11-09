package repositories

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
)

type ExecutionListFilter struct {
	OwnerID string
	TaskID  *string
	Status  *execution.Status
	Limit   int
	Cursor  string
	From    *time.Time
	To      *time.Time
}

type ExecutionPage struct {
	Items      []execution.Execution
	NextCursor string
}

// ExecutionRepository persists execution records (read-only surface in WP2.3).
type ExecutionRepository interface {
	Create(ctx context.Context, exec execution.Execution) error
	GetByID(ctx context.Context, ownerID, executionID string) (execution.Execution, error)
	GetByExecutionID(ctx context.Context, executionID string) (execution.Execution, error)
	List(ctx context.Context, filter ExecutionListFilter) (ExecutionPage, error)
	UpdateStatus(ctx context.Context, executionID string, status execution.Status, expectedStatus execution.Status) error
	SetResult(ctx context.Context, executionID string, resultRef *string, errorCode, errorMessage *string) error
	SetWorker(ctx context.Context, executionID, workerID string, attempt int, leaseGen int) error
	SetRedriveCount(ctx context.Context, executionID string, count int) error
	GetRedriveCount(ctx context.Context, executionID string) (int, error)
	EnqueueRequeue(ctx context.Context, executionID string, availableAt time.Time) error
}
