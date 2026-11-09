package leasing

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	portclock "github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type Service struct {
	queue repositories.QueueRepository
	exec  repositories.ExecutionRepository
	clock portclock.Clock
}

func NewService(q repositories.QueueRepository, e repositories.ExecutionRepository, c portclock.Clock) *Service {
	return &Service{queue: q, exec: e, clock: c}
}

type RenewInput struct {
	ExecutionID     string
	LeaseGeneration int
	NewExpiry       time.Time
}

func (s *Service) Renew(ctx context.Context, in RenewInput) error {
	if err := s.queue.RenewLease(ctx, in.ExecutionID, in.LeaseGeneration, in.NewExpiry); err != nil {
		return apperr.Wrap(apperr.CodeLeaseExpired, "renew lease", err)
	}
	return nil
}

func (s *Service) Reap(ctx context.Context, now time.Time) error {
	msgs, err := s.queue.ReapExpired(ctx, now)
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, "reap", err)
	}
	for _, m := range msgs {
		ex, err := s.exec.GetByExecutionID(ctx, m.ExecutionID)
		if err != nil {
			continue
		}
		if ex.Attempt < ex.MaxAttempts {
			_ = s.exec.UpdateStatus(ctx, m.ExecutionID, execution.StatusRetrying, execution.StatusRunning)
		} else {
			_ = s.exec.UpdateStatus(ctx, m.ExecutionID, execution.StatusDeadLetter, execution.StatusRunning)
		}
	}
	return nil
}
