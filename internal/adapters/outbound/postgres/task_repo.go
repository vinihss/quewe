package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type TaskRepository struct{ pool *pgxpool.Pool }

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository { return &TaskRepository{pool: pool} }

func (r *TaskRepository) Create(ctx context.Context, def task.Definition) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	// Auto-create principal if not exists (dev mode convenience for debug auth stub).
	if _, err := tx.Exec(ctx, `INSERT INTO principals (id, kind, name, active) VALUES ($1::uuid, 'user', $2, true) ON CONFLICT (id) DO NOTHING`, def.OwnerID, def.OwnerID); err != nil {
		return fmt.Errorf("auto-create principal: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO tasks (id,owner_id,name,type,description,payload,target_capability,target_worker_id,timeout_seconds,max_retries,priority,enabled,deleted_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		def.ID, def.OwnerID, def.Name, def.Type, def.Description, def.Payload, def.Target.Capability, def.Target.WorkerID, def.ExecutionPolicy.TimeoutSeconds, def.ExecutionPolicy.MaxRetries, def.ExecutionPolicy.Priority, def.Enabled, def.DeletedAt, def.CreatedAt, def.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return repositories.ErrConflict
		}
		return fmt.Errorf("insert task: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO task_schedules (task_id,type,expression,timezone,run_at,next_run_at,last_run_at,misfire_policy,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		def.ID, string(def.Schedule.Type), def.Schedule.Expression, def.Schedule.Timezone, def.Schedule.RunAt, def.Schedule.NextRunAt, def.Schedule.LastRunAt, string(def.Schedule.MisfirePolicy), def.CreatedAt, def.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert schedule: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *TaskRepository) GetByID(ctx context.Context, ownerID, id string) (task.Definition, error) {
	row := r.pool.QueryRow(ctx, `SELECT id,owner_id,name,type,description,payload,target_capability,target_worker_id,timeout_seconds,max_retries,priority,enabled,deleted_at,created_at,updated_at FROM tasks WHERE id=$1 AND owner_id=$2`, id, ownerID)
	var d task.Definition
	var cap string
	var workerID *string
	err := row.Scan(&d.ID, &d.OwnerID, &d.Name, &d.Type, &d.Description, &d.Payload, &cap, &workerID, &d.ExecutionPolicy.TimeoutSeconds, &d.ExecutionPolicy.MaxRetries, &d.ExecutionPolicy.Priority, &d.Enabled, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return task.Definition{}, repositories.ErrNotFound
	}
	if err != nil {
		return task.Definition{}, fmt.Errorf("get task: %w", err)
	}
	d.Target.Capability = cap
	d.Target.WorkerID = workerID
	// schedule
	srow := r.pool.QueryRow(ctx, `SELECT type,expression,timezone,run_at,next_run_at,last_run_at,misfire_policy FROM task_schedules WHERE task_id=$1`, id)
	var st string
	var mp string
	err = srow.Scan(&st, &d.Schedule.Expression, &d.Schedule.Timezone, &d.Schedule.RunAt, &d.Schedule.NextRunAt, &d.Schedule.LastRunAt, &mp)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return task.Definition{}, fmt.Errorf("get schedule: %w", err)
	}
	d.Schedule.Type = task.ScheduleType(st)
	d.Schedule.MisfirePolicy = task.MisfirePolicy(mp)
	return d, nil
}

func (r *TaskRepository) GetByTaskID(ctx context.Context, taskID string) (task.Definition, error) {
	row := r.pool.QueryRow(ctx, `SELECT id,owner_id,name,type,description,payload,target_capability,target_worker_id,timeout_seconds,max_retries,priority,enabled,deleted_at,created_at,updated_at FROM tasks WHERE id=$1`, taskID)
	var d task.Definition
	var cap string
	var workerID *string
	err := row.Scan(&d.ID, &d.OwnerID, &d.Name, &d.Type, &d.Description, &d.Payload, &cap, &workerID, &d.ExecutionPolicy.TimeoutSeconds, &d.ExecutionPolicy.MaxRetries, &d.ExecutionPolicy.Priority, &d.Enabled, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return task.Definition{}, repositories.ErrNotFound
	}
	if err != nil {
		return task.Definition{}, fmt.Errorf("get task: %w", err)
	}
	d.Target.Capability = cap
	d.Target.WorkerID = workerID
	srow := r.pool.QueryRow(ctx, `SELECT type,expression,timezone,run_at,next_run_at,last_run_at,misfire_policy FROM task_schedules WHERE task_id=$1`, taskID)
	var st string
	var mp string
	err = srow.Scan(&st, &d.Schedule.Expression, &d.Schedule.Timezone, &d.Schedule.RunAt, &d.Schedule.NextRunAt, &d.Schedule.LastRunAt, &mp)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return task.Definition{}, fmt.Errorf("get schedule: %w", err)
	}
	d.Schedule.Type = task.ScheduleType(st)
	d.Schedule.MisfirePolicy = task.MisfirePolicy(mp)
	return d, nil
}

func (r *TaskRepository) List(ctx context.Context, filter repositories.TaskListFilter) (repositories.TaskPage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `SELECT id,owner_id,name,type,description,payload,target_capability,target_worker_id,timeout_seconds,max_retries,priority,enabled,deleted_at,created_at,updated_at FROM tasks WHERE owner_id=$1 ORDER BY created_at DESC LIMIT $2`, filter.OwnerID, limit)
	if err != nil {
		return repositories.TaskPage{}, err
	}
	defer rows.Close()
	var items []task.Definition
	for rows.Next() {
		var d task.Definition
		var cap string
		var workerID *string
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Name, &d.Type, &d.Description, &d.Payload, &cap, &workerID, &d.ExecutionPolicy.TimeoutSeconds, &d.ExecutionPolicy.MaxRetries, &d.ExecutionPolicy.Priority, &d.Enabled, &d.DeletedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return repositories.TaskPage{}, err
		}
		d.Target.Capability = cap
		d.Target.WorkerID = workerID
		items = append(items, d)
	}
	return repositories.TaskPage{Items: items}, nil
}

func isUnique(err error) bool {
	return err != nil && (contains(err.Error(), "unique") || contains(err.Error(), "duplicate"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
