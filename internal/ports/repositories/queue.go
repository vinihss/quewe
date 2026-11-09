package repositories

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/queue"
)

type QueueRepository interface {
	Enqueue(ctx context.Context, msg queue.QueueMessage) error
	Reserve(ctx context.Context, workerID string, capabilities []string, max int) ([]queue.QueueMessage, error)
	RenewLease(ctx context.Context, executionID string, gen int, newExpiry time.Time) error
	Ack(ctx context.Context, executionID string, gen int) error
	Nack(ctx context.Context, executionID string, gen int) error
	ReapExpired(ctx context.Context, now time.Time) ([]queue.QueueMessage, error)
	Release(ctx context.Context, executionID string, gen int) error
	GetByExecution(ctx context.Context, executionID string) (queue.QueueMessage, error)
}
