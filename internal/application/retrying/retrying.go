package retrying

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

// Retry verifica se pode retry e agenda requeue
func Retry(ctx context.Context, execRepo repositories.ExecutionRepository, execID string, backoff time.Duration) error {
	exec, err := execRepo.GetByID(ctx, "", execID)
	if err != nil {
		return err
	}

	if exec.Attempt >= exec.MaxAttempts {
		return apperr.New(apperr.CodeInvalidTransition, "max attempts reached; use DeadLetter")
	}

	// incrementa attempts
	exec.Attempt++
	if err := execRepo.UpdateStatus(ctx, execID, execution.StatusRetrying, execution.StatusRetrying); err != nil {
		return err
	}

	// agenda requeue com backoff
	availableAt := time.Now().Add(backoff)
	if err := execRepo.EnqueueRequeue(ctx, execID, availableAt); err != nil {
		return err
	}

	return nil
}

// DeadLetter move execução para status dead_letter
func DeadLetter(ctx context.Context, execRepo repositories.ExecutionRepository, execID string) error {
	exec, err := execRepo.GetByID(ctx, "", execID)
	if err != nil {
		return err
	}

	exec.Status = execution.StatusDeadLetter
	exec.RedriveCount++

	return execRepo.UpdateStatus(ctx, execID, execution.StatusDeadLetter, execution.StatusDeadLetter)
}

// Redrive permite redrive manual (máx 3 por ADR-013)
func Redrive(ctx context.Context, execRepo repositories.ExecutionRepository, execID string) error {
	exec, err := execRepo.GetByID(ctx, "", execID)
	if err != nil {
		return err
	}

	if exec.RedriveCount >= 3 {
		return apperr.New(apperr.CodeConflict, "redrive limit exceeded; max 3 per execution per ADR-013")
	}

	exec.RedriveCount++
	exec.Status = execution.StatusQueued // reposiciona para queued

	return execRepo.UpdateStatus(ctx, execID, execution.StatusQueued, execution.StatusQueued)
}
