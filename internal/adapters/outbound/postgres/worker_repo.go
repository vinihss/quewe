package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinihss/quewe/services/quewe/internal/domain/worker"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type WorkerRepository struct{ pool *pgxpool.Pool }

func NewWorkerRepository(pool *pgxpool.Pool) *WorkerRepository {
	return &WorkerRepository{pool: pool}
}

func (r *WorkerRepository) Register(ctx context.Context, w worker.Worker, capabilities []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	md, _ := json.Marshal(w.Metadata)
	_, err = tx.Exec(ctx, `INSERT INTO workers (worker_id,hostname,version,status,concurrency,last_heartbeat_at,registered_at,metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (worker_id) DO UPDATE SET hostname=EXCLUDED.hostname, version=EXCLUDED.version, status=EXCLUDED.status, concurrency=EXCLUDED.concurrency, metadata=EXCLUDED.metadata`,
		w.WorkerID, w.Hostname, w.Version, string(w.Status), w.Concurrency, w.LastHeartbeatAt, w.RegisteredAt, md)
	if err != nil {
		return fmt.Errorf("register worker: %w", err)
	}
	_, err = tx.Exec(ctx, `DELETE FROM worker_capabilities WHERE worker_id=$1`, w.WorkerID)
	if err != nil {
		return err
	}
	for _, c := range capabilities {
		_, err = tx.Exec(ctx, `INSERT INTO worker_capabilities (worker_id,capability) VALUES ($1,$2)`, w.WorkerID, c)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *WorkerRepository) Heartbeat(ctx context.Context, workerID string, now time.Time, metrics map[string]any) error {
	md, _ := json.Marshal(metrics)
	ct, err := r.pool.Exec(ctx, `UPDATE workers SET last_heartbeat_at=$1, metadata=$2 WHERE worker_id=$3`, now, md, workerID)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

func (r *WorkerRepository) Get(ctx context.Context, workerID string) (worker.Worker, error) {
	row := r.pool.QueryRow(ctx, `SELECT worker_id,hostname,version,status,concurrency,last_heartbeat_at,registered_at,metadata FROM workers WHERE worker_id=$1`, workerID)
	w, err := scanWorker(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return worker.Worker{}, repositories.ErrNotFound
	}
	if err != nil {
		return worker.Worker{}, fmt.Errorf("get worker: %w", err)
	}
	caps, _ := r.loadCaps(ctx, workerID)
	w.Capabilities = caps
	return w, nil
}

func (r *WorkerRepository) loadCaps(ctx context.Context, workerID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT capability FROM worker_capabilities WHERE worker_id=$1`, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var caps []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		caps = append(caps, c)
	}
	return caps, rows.Err()
}

func scanWorker(row pgx.Row) (worker.Worker, error) {
	var w worker.Worker
	var status string
	var md []byte
	var hb *time.Time
	err := row.Scan(&w.WorkerID, &w.Hostname, &w.Version, &status, &w.Concurrency, &hb, &w.RegisteredAt, &md)
	if err != nil {
		return worker.Worker{}, err
	}
	w.Status = worker.Status(status)
	if hb != nil {
		w.LastHeartbeatAt = *hb
	}
	if len(md) > 0 {
		_ = json.Unmarshal(md, &w.Metadata)
	}
	return w, nil
}

func (r *WorkerRepository) List(ctx context.Context) ([]worker.Worker, error) {
	rows, err := r.pool.Query(ctx, `SELECT worker_id,hostname,version,status,concurrency,last_heartbeat_at,registered_at,metadata FROM workers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []worker.Worker
	for rows.Next() {
		w, err := scanWorker(rows)
		if err != nil {
			return nil, err
		}
		caps, _ := r.loadCaps(ctx, w.WorkerID)
		w.Capabilities = caps
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WorkerRepository) SetStatus(ctx context.Context, workerID string, status worker.Status) error {
	ct, err := r.pool.Exec(ctx, `UPDATE workers SET status=$1 WHERE worker_id=$2`, string(status), workerID)
	if err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

var _ repositories.WorkerRepository = (*WorkerRepository)(nil)
