package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type ExecutionRepository struct{ pool *pgxpool.Pool }

func NewExecutionRepository(pool *pgxpool.Pool) *ExecutionRepository {
	return &ExecutionRepository{pool: pool}
}

func (r *ExecutionRepository) Create(ctx context.Context, exec execution.Execution) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO executions (id, owner_id, task_id, status, attempt, max_attempts, scheduled_for, worker_id, started_at, finished_at, cancel_requested_at, redrive_count, result_ref, error_code, error_message, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, now(), now())`,
		exec.ID, exec.OwnerID, exec.TaskID, exec.Status, exec.Attempt, exec.MaxAttempts, exec.ScheduledFor, exec.WorkerID, exec.StartedAt, exec.FinishedAt, exec.CancelRequestedAt, exec.RedriveCount, exec.ResultRef, exec.ErrorCode, exec.ErrorMessage)
	return err
}

func (r *ExecutionRepository) GetByID(ctx context.Context, ownerID, execID string) (execution.Execution, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, owner_id, task_id, status, attempt, max_attempts, scheduled_for, worker_id, started_at, finished_at, cancel_requested_at, redrive_count, result_ref, error_code, error_message, created_at, updated_at FROM executions WHERE id = $1`, execID)
	var e execution.Execution
	var redriveCount sql.NullInt32
	err := row.Scan(&e.ID, &e.OwnerID, &e.TaskID, &e.Status, &e.Attempt, &e.MaxAttempts, &e.ScheduledFor, &e.WorkerID, &e.StartedAt, &e.FinishedAt, &e.CancelRequestedAt, &redriveCount, &e.ResultRef, &e.ErrorCode, &e.ErrorMessage, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return execution.Execution{}, err
	}
	e.RedriveCount = int(redriveCount.Int32)
	return e, nil
}

func (r *ExecutionRepository) GetByExecutionID(ctx context.Context, executionID string) (execution.Execution, error) {
	return r.GetByID(ctx, "", executionID)
}

func (r *ExecutionRepository) List(ctx context.Context, filter repositories.ExecutionListFilter) (repositories.ExecutionPage, error) {
	query := `SELECT id, owner_id, task_id, status, attempt, max_attempts, scheduled_for, worker_id, started_at, finished_at, cancel_requested_at, redrive_count, result_ref, error_code, error_message, created_at, updated_at FROM executions WHERE 1=1`
	args := make([]interface{}, 0)
	argID := 1

	if filter.OwnerID != "" {
		query += ` AND owner_id = $` + fmt.Sprintf("%d", argID)
		args = append(args, filter.OwnerID)
		argID++
	}
	if filter.TaskID != nil {
		query += ` AND task_id = $` + fmt.Sprintf("%d", argID)
		args = append(args, *filter.TaskID)
		argID++
	}
	if filter.Status != nil {
		query += ` AND status = $` + fmt.Sprintf("%d", argID)
		args = append(args, string(*filter.Status))
		argID++
	}
	if filter.Limit > 0 {
		query += ` LIMIT $` + fmt.Sprintf("%d", argID)
		args = append(args, filter.Limit)
		argID++
	}
	if filter.Cursor != "" {
		query += ` AND id > $` + fmt.Sprintf("%d", argID)
		args = append(args, filter.Cursor)
		argID++
	}
	if filter.From != nil {
		query += ` AND created_at > $` + fmt.Sprintf("%d", argID)
		args = append(args, filter.From)
		argID++
	}
	if filter.To != nil {
		query += ` AND created_at < $` + fmt.Sprintf("%d", argID)
		args = append(args, filter.To)
		argID++
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return repositories.ExecutionPage{}, err
	}
	defer rows.Close()

	var items []execution.Execution
	for rows.Next() {
		var e execution.Execution
		var rc sql.NullInt32
		if err := rows.Scan(&e.ID, &e.OwnerID, &e.TaskID, &e.Status, &e.Attempt, &e.MaxAttempts, &e.ScheduledFor, &e.WorkerID, &e.StartedAt, &e.FinishedAt, &e.CancelRequestedAt, &rc, &e.ResultRef, &e.ErrorCode, &e.ErrorMessage, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return repositories.ExecutionPage{}, err
		}
		e.RedriveCount = int(rc.Int32)
		items = append(items, e)
	}

	return repositories.ExecutionPage{Items: items}, nil
}

func (r *ExecutionRepository) SetWorker(ctx context.Context, executionID, workerID string, attempt int, leaseGen int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE executions SET worker_id = $1, attempt = $2, updated_at = now() WHERE id = $3`, workerID, attempt, executionID)
	return err
}

func (r *ExecutionRepository) UpdateStatus(ctx context.Context, execID string, status execution.Status, expectedStatus execution.Status) error {
	// compare-and-set: apenas se status atual = expectedStatus
	_, err := r.pool.Exec(ctx,
		`UPDATE executions SET status = $1, updated_at = now() WHERE id = $2 AND status = $3`, status, execID, expectedStatus)
	return err
}

func (r *ExecutionRepository) SetResult(ctx context.Context, execID string, resultRef *string, errorCode, errorMessage *string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE executions SET result_ref = $1, error_code = $2, error_message = $3, updated_at = now() WHERE id = $3`, resultRef, errorCode, errorMessage, execID)
	return err
}

func (r *ExecutionRepository) SetRedriveCount(ctx context.Context, execID string, count int) error {
	_, err := r.pool.Exec(ctx, `UPDATE executions SET redrive_count = $1 WHERE id = $2`, count, execID)
	return err
}

func (r *ExecutionRepository) GetRedriveCount(ctx context.Context, execID string) (int, error) {
	var count int
	_ = r.pool.QueryRow(ctx, `SELECT redrive_count FROM executions WHERE id = $1`, execID).Scan(&count)
	return count, nil
}

func (r *ExecutionRepository) EnqueueRequeue(ctx context.Context, execID string, availableAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE executions SET available_at = $1, status = 'queued', updated_at = now() WHERE id = $2`, availableAt, execID)
	return err
}
