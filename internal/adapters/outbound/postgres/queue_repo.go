package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinihss/quewe/services/quewe/internal/domain/queue"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type QueueRepository struct{ pool *pgxpool.Pool }

func NewQueueRepository(pool *pgxpool.Pool) *QueueRepository {
	return &QueueRepository{pool: pool}
}

func (r *QueueRepository) Enqueue(ctx context.Context, msg queue.QueueMessage) error {
	tc, _ := json.Marshal(msg.TraceContext)
	_, err := r.pool.Exec(ctx, `INSERT INTO queue_messages (id,execution_id,queue,capability,priority,available_at,status,attempts,reserved_by,lease_expires_at,lease_generation,trace_context,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		msg.ID, msg.ExecutionID, msg.Queue, msg.Capability, msg.Priority, msg.AvailableAt, string(msg.Status), msg.Attempts, msg.ReservedBy, msg.LeaseExpiresAt, msg.LeaseGeneration, tc, msg.CreatedAt, msg.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return repositories.ErrConflict
		}
		return fmt.Errorf("enqueue: %w", err)
	}
	return nil
}

func scanQueue(row pgx.Row) (queue.QueueMessage, error) {
	var m queue.QueueMessage
	var status string
	var tc []byte
	var reservedBy *string
	var leaseExp *time.Time
	err := row.Scan(&m.ID, &m.ExecutionID, &m.Queue, &m.Capability, &m.Priority, &m.AvailableAt, &status, &m.Attempts, &reservedBy, &leaseExp, &m.LeaseGeneration, &tc, &m.CreatedAt, &m.CreatedAt)
	if err != nil {
		return queue.QueueMessage{}, err
	}
	m.Status = queue.Status(status)
	m.ReservedBy = reservedBy
	m.LeaseExpiresAt = leaseExp
	if len(tc) > 0 {
		_ = json.Unmarshal(tc, &m.TraceContext)
	}
	return m, nil
}

func (r *QueueRepository) Reserve(ctx context.Context, workerID string, capabilities []string, max int) ([]queue.QueueMessage, error) {
	if max <= 0 {
		max = 1
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,execution_id,queue,capability,priority,available_at,status,attempts,reserved_by,lease_expires_at,lease_generation,trace_context,created_at,updated_at FROM queue_messages WHERE status='queued' AND capability = ANY($1) AND available_at <= now() ORDER BY priority DESC, available_at ASC, id ASC LIMIT $2 FOR UPDATE SKIP LOCKED`, capabilities, max)
	if err != nil {
		return nil, fmt.Errorf("reserve select: %w", err)
	}
	var msgs []queue.QueueMessage
	for rows.Next() {
		m, err := scanQueue(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		msgs = append(msgs, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]queue.QueueMessage, 0, len(msgs))
	for _, m := range msgs {
		gen := m.LeaseGeneration + 1
		exp := time.Now().UTC().Add(60 * time.Second)
		ct, err := tx.Exec(ctx, `UPDATE queue_messages SET status='reserved', reserved_by=$1, lease_expires_at=$2, lease_generation=$3, attempts=attempts+1, updated_at=now() WHERE id=$4 AND status='queued'`, workerID, exp, gen, m.ID)
		if err != nil {
			return nil, err
		}
		if ct.RowsAffected() == 0 {
			continue
		}
		m.Status = queue.StatusReserved
		w := workerID
		m.ReservedBy = &w
		m.LeaseGeneration = gen
		t := exp
		m.LeaseExpiresAt = &t
		m.Attempts++
		out = append(out, m)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *QueueRepository) RenewLease(ctx context.Context, executionID string, gen int, newExpiry time.Time) error {
	ct, err := r.pool.Exec(ctx, `UPDATE queue_messages SET lease_expires_at=$1, updated_at=now() WHERE execution_id=$2 AND lease_generation=$3 AND status='reserved'`, newExpiry, executionID, gen)
	if err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

func (r *QueueRepository) Ack(ctx context.Context, executionID string, gen int) error {
	ct, err := r.pool.Exec(ctx, `UPDATE queue_messages SET status='completed', updated_at=now() WHERE execution_id=$1 AND lease_generation=$2 AND status='reserved'`, executionID, gen)
	if err != nil {
		return fmt.Errorf("ack: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

func (r *QueueRepository) Nack(ctx context.Context, executionID string, gen int) error {
	ct, err := r.pool.Exec(ctx, `UPDATE queue_messages SET status='queued', reserved_by=NULL, lease_expires_at=NULL, updated_at=now() WHERE execution_id=$1 AND lease_generation=$2 AND status='reserved'`, executionID, gen)
	if err != nil {
		return fmt.Errorf("nack: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

func (r *QueueRepository) ReapExpired(ctx context.Context, now time.Time) ([]queue.QueueMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,execution_id,queue,capability,priority,available_at,status,attempts,reserved_by,lease_expires_at,lease_generation,trace_context,created_at,updated_at FROM queue_messages WHERE status='reserved' AND lease_expires_at < $1 FOR UPDATE SKIP LOCKED`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []queue.QueueMessage
	for rows.Next() {
		m, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		// Release the lease: set status back to queued, clear reserved_by and lease_expires_at, increment attempts
		_, err = r.pool.Exec(ctx, `UPDATE queue_messages SET status='queued', reserved_by=NULL, lease_expires_at=NULL, attempts=attempts+1, updated_at=now() WHERE id=$1`, m.ID)
		if err != nil {
			return nil, err
		}
		m.Status = queue.StatusQueued
		m.ReservedBy = nil
		m.LeaseExpiresAt = nil
		m.Attempts++
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *QueueRepository) Release(ctx context.Context, executionID string, gen int) error {
	ct, err := r.pool.Exec(ctx, `UPDATE queue_messages SET status='queued', reserved_by=NULL, lease_expires_at=NULL, updated_at=now() WHERE execution_id=$1 AND lease_generation=$2 AND status='reserved'`, executionID, gen)
	if err != nil {
		return fmt.Errorf("release: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

func (r *QueueRepository) GetByExecution(ctx context.Context, executionID string) (queue.QueueMessage, error) {
	row := r.pool.QueryRow(ctx, `SELECT id,execution_id,queue,capability,priority,available_at,status,attempts,reserved_by,lease_expires_at,lease_generation,trace_context,created_at,updated_at FROM queue_messages WHERE execution_id=$1`, executionID)
	m, err := scanQueue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return queue.QueueMessage{}, repositories.ErrNotFound
	}
	if err != nil {
		return queue.QueueMessage{}, fmt.Errorf("get by execution: %w", err)
	}
	return m, nil
}

var _ repositories.QueueRepository = (*QueueRepository)(nil)
var _ = pgconn.PgError{}
