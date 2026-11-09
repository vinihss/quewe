package repositories

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
)

// ScheduleRepository persists task schedules.
type ScheduleRepository interface {
	FindDue(ctx context.Context, now time.Time) ([]task.Definition, error)
	AdvanceSchedule(ctx context.Context, taskID string, lastRunAt, nextRunAt *time.Time) error
	GetSchedule(ctx context.Context, taskID string) (task.Schedule, error)
}
