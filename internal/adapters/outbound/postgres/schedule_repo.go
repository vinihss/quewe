package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type ScheduleRepository struct{ pool *pgxpool.Pool }

func NewScheduleRepository(pool *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{pool: pool}
}

func (r *ScheduleRepository) FindDue(ctx context.Context, now time.Time) ([]task.Definition, error) {
	rows, err := r.pool.Query(ctx, `SELECT t.id,t.owner_id,t.name,t.type,t.description,t.payload,t.target_capability,t.target_worker_id,t.timeout_seconds,t.max_retries,t.priority,t.enabled,t.deleted_at,t.created_at,t.updated_at FROM tasks t JOIN task_schedules s ON s.task_id=t.id WHERE t.enabled=true AND t.deleted_at IS NULL AND s.next_run_at IS NOT NULL AND s.next_run_at <= $1 ORDER BY s.next_run_at`, now)
	if err != nil {
		return nil, fmt.Errorf("find due: %w", err)
	}
	defer rows.Close()
	var out []task.Definition
	for rows.Next() {
		var d task.Definition
		var cap string
		var workerID *string
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Name, &d.Type, &d.Description, &d.Payload, &cap, &workerID, &d.ExecutionPolicy.TimeoutSeconds, &d.ExecutionPolicy.MaxRetries, &d.ExecutionPolicy.Priority, &d.Enabled, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan due: %w", err)
		}
		d.Target.Capability = cap
		d.Target.WorkerID = workerID
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *ScheduleRepository) GetSchedule(ctx context.Context, taskID string) (task.Schedule, error) {
	row := r.pool.QueryRow(ctx, `SELECT type,expression,timezone,run_at,next_run_at,last_run_at,misfire_policy FROM task_schedules WHERE task_id=$1`, taskID)
	var s task.Schedule
	var st string
	var mp string
	err := row.Scan(&st, &s.Expression, &s.Timezone, &s.RunAt, &s.NextRunAt, &s.LastRunAt, &mp)
	if errors.Is(err, pgx.ErrNoRows) {
		return task.Schedule{}, repositories.ErrNotFound
	}
	if err != nil {
		return task.Schedule{}, fmt.Errorf("get schedule: %w", err)
	}
	s.Type = task.ScheduleType(st)
	s.MisfirePolicy = task.MisfirePolicy(mp)
	return s, nil
}

func (r *ScheduleRepository) AdvanceSchedule(ctx context.Context, taskID string, lastRunAt, nextRunAt *time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE task_schedules SET last_run_at=$2, next_run_at=$3, updated_at=now() WHERE task_id=$1`, taskID, lastRunAt, nextRunAt)
	if err != nil {
		return fmt.Errorf("advance schedule: %w", err)
	}
	return nil
}
