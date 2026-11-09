package broker

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/queue"
)

type ResultRef struct {
	ContentType string
	InlineBody  []byte
	ArtifactURI string
	SizeBytes   int64
	Checksum    string
}

type ExecutionError struct {
	Code      string
	Message   string
	Retryable bool
}

type MessageBroker interface {
	Publish(ctx context.Context, msg queue.QueueMessage) error
	Reserve(ctx context.Context, workerID string, capabilities []string, max int, wait time.Duration) ([]queue.QueueMessage, error)
	RenewLease(ctx context.Context, executionID string, gen int, newExpiry time.Time) error
	Release(ctx context.Context, executionID string, gen int) error
	Ack(ctx context.Context, executionID string, gen int, result ResultRef) error
	Nack(ctx context.Context, executionID string, gen int, err ExecutionError, retryable bool) error
}
