package repositories

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/worker"
)

type WorkerRepository interface {
	Register(ctx context.Context, w worker.Worker, capabilities []string) error
	Heartbeat(ctx context.Context, workerID string, now time.Time, metrics map[string]any) error
	Get(ctx context.Context, workerID string) (worker.Worker, error)
	List(ctx context.Context) ([]worker.Worker, error)
	SetStatus(ctx context.Context, workerID string, status worker.Status) error
}
