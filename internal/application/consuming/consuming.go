package consuming

import (
	"context"
	"errors"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/domain/queue"
	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	portclock "github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type Service struct {
	queue repositories.QueueRepository
	exec  repositories.ExecutionRepository
	tasks repositories.TaskRepository
	clock portclock.Clock
}

func NewService(q repositories.QueueRepository, e repositories.ExecutionRepository, t repositories.TaskRepository, c portclock.Clock) *Service {
	return &Service{queue: q, exec: e, tasks: t, clock: c}
}

type QueueMessageEnvelope struct {
	MessageID       string
	ExecutionID     string
	TaskID          string
	Type            string
	Capability      string
	Attempt         int
	MaxAttempts     int
	LeaseGeneration int
	LeaseExpiresAt  time.Time
	TimeoutSeconds  int
	IdempotencyKey  string
	Payload         []byte
	TraceContext    map[string]string
}

type ConsumeInput struct {
	WorkerID     string
	Capabilities []string
	Max          int
	Wait         time.Duration
}

type ConsumeOutput struct {
	Messages []QueueMessageEnvelope
}

func (s *Service) Consume(ctx context.Context, in ConsumeInput) (ConsumeOutput, error) {
	if in.Max <= 0 {
		in.Max = 1
	}
	msgs, err := s.queue.Reserve(ctx, in.WorkerID, in.Capabilities, in.Max)
	if err != nil {
		return ConsumeOutput{}, apperr.Wrap(apperr.CodeInternal, "reserve", err)
	}
	if len(msgs) == 0 && in.Wait > 0 {
		select {
		case <-time.After(in.Wait):
		case <-ctx.Done():
			return ConsumeOutput{}, ctx.Err()
		}
		msgs, err = s.queue.Reserve(ctx, in.WorkerID, in.Capabilities, in.Max)
		if err != nil {
			return ConsumeOutput{}, apperr.Wrap(apperr.CodeInternal, "reserve", err)
		}
	}
	out := ConsumeOutput{}
	for _, m := range msgs {
		ex, err := s.exec.GetByExecutionID(ctx, m.ExecutionID)
		if err != nil {
			return ConsumeOutput{}, apperr.Wrap(apperr.CodeNotFound, "execution", err)
		}
		tdef, err := s.tasks.GetByTaskID(ctx, ex.TaskID)
		if err != nil {
			return ConsumeOutput{}, apperr.Wrap(apperr.CodeNotFound, "task", err)
		}
		leaseExp := time.Time{}
		if m.LeaseExpiresAt != nil {
			leaseExp = *m.LeaseExpiresAt
		}
		env := QueueMessageEnvelope{
			MessageID:       m.ID,
			ExecutionID:     m.ExecutionID,
			TaskID:          ex.TaskID,
			Type:            tdef.Type,
			Capability:      m.Capability,
			Attempt:         ex.Attempt,
			MaxAttempts:     ex.MaxAttempts,
			LeaseGeneration: m.LeaseGeneration,
			LeaseExpiresAt:  leaseExp,
			TimeoutSeconds:  tdef.ExecutionPolicy.TimeoutSeconds,
			Payload:         tdef.Payload,
			TraceContext:    m.TraceContext,
		}
		out.Messages = append(out.Messages, env)
	}
	return out, nil
}

type AckInput struct {
	ExecutionID     string
	LeaseGeneration int
	Result          ResultRef
}

type ResultRef struct {
	InlineBody  *string
	ArtifactURI *string
}

func (s *Service) Ack(ctx context.Context, in AckInput) error {
	ex, err := s.exec.GetByExecutionID(ctx, in.ExecutionID)
	if err != nil {
		return apperr.Wrap(apperr.CodeNotFound, "execution", err)
	}
	if ex.Status == execution.StatusCompleted {
		// idempotent if same result
		return nil
	}
	if err := s.exec.SetResult(ctx, in.ExecutionID, in.Result.InlineBody, nil, nil); err != nil {
		// ignore; still update status
	}
	if err := s.queue.Ack(ctx, in.ExecutionID, in.LeaseGeneration); err != nil {
		return apperr.Wrap(apperr.CodeLeaseExpired, "ack lease", err)
	}
	if err := s.exec.UpdateStatus(ctx, in.ExecutionID, execution.StatusCompleted, execution.StatusRunning); err != nil {
		return apperr.Wrap(apperr.CodeInvalidTransition, "ack status", err)
	}
	_ = ex
	return nil
}

type FinishResult struct {
	InlineBody  *string
	ArtifactURI *string
	Success     bool
}

type FinishInput struct {
	ExecutionID     string
	LeaseGeneration int
	Result          FinishResult
}

func (s *Service) Finish(ctx context.Context, in FinishInput) error {
	ex, err := s.exec.GetByExecutionID(ctx, in.ExecutionID)
	if err != nil {
		return apperr.Wrap(apperr.CodeNotFound, "execution", err)
	}
	if ex.Status == execution.StatusCompleted {
		return nil
	}
	if !in.Result.Success {
		ec, em := parseErrorCode("error_code"), parseErrorMessage("error_message")
		_ = s.exec.SetResult(ctx, in.ExecutionID, in.Result.ArtifactURI, ec, em)
		_ = s.exec.UpdateStatus(ctx, in.ExecutionID, execution.StatusFailed, execution.StatusRunning)
	} else {
		inline, _ := in.Result.InlineBody, in.Result.ArtifactURI
		_ = s.exec.SetResult(ctx, in.ExecutionID, inline, nil, nil)
		_ = s.queue.Ack(ctx, in.ExecutionID, in.LeaseGeneration)
		_ = s.exec.UpdateStatus(ctx, in.ExecutionID, execution.StatusCompleted, execution.StatusRunning)
	}
	_ = ex
	return nil
}

func parseErrorCode(code string) *string {
	return &code
}

func parseErrorMessage(msg string) *string {
	return &msg
}

type ExecutionError struct {
	Code    string
	Message string
}

type NackInput struct {
	ExecutionID     string
	LeaseGeneration int
	Error           ExecutionError
	Retryable       bool
}

func (s *Service) Nack(ctx context.Context, in NackInput) error {
	ex, err := s.exec.GetByExecutionID(ctx, in.ExecutionID)
	if err != nil {
		return apperr.Wrap(apperr.CodeNotFound, "execution", err)
	}
	if in.Retryable && ex.Attempt < ex.MaxAttempts {
		_ = s.exec.UpdateStatus(ctx, in.ExecutionID, execution.StatusRetrying, execution.StatusRunning)
		_ = s.queue.Nack(ctx, in.ExecutionID, in.LeaseGeneration)
		return nil
	}
	_ = s.exec.UpdateStatus(ctx, in.ExecutionID, execution.StatusFailed, execution.StatusRunning)
	_ = s.queue.Nack(ctx, in.ExecutionID, in.LeaseGeneration)
	_ = errors.New("")
	return nil
}

type CancelInput struct {
	ExecutionID string
}

func (s *Service) Cancel(ctx context.Context, in CancelInput) error {
	ex, err := s.exec.GetByExecutionID(ctx, in.ExecutionID)
	if err != nil {
		return apperr.Wrap(apperr.CodeNotFound, "execution", err)
	}
	if ex.Status == execution.StatusPending || ex.Status == execution.StatusQueued {
		if err := s.exec.UpdateStatus(ctx, in.ExecutionID, execution.StatusCancelled, ex.Status); err != nil {
			return apperr.Wrap(apperr.CodeInvalidTransition, "cancel", err)
		}
		_ = s.queue.Release(ctx, in.ExecutionID, 0)
		return nil
	}
	if ex.Status == execution.StatusRunning {
		now := s.clock.Now()
		ex.CancelRequestedAt = &now
		// best-effort: no status change
		return nil
	}
	return apperr.New(apperr.CodeInvalidTransition, "cannot cancel")
}

var _ = task.Definition{}
var _ = queue.QueueMessage{}
