package workers

import (
	"context"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/domain/worker"
	portclock "github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type Service struct {
	repo  repositories.WorkerRepository
	exec  repositories.ExecutionRepository
	clock portclock.Clock
}

func NewService(w repositories.WorkerRepository, e repositories.ExecutionRepository, c portclock.Clock) *Service {
	return &Service{repo: w, exec: e, clock: c}
}

type RegisterInput struct {
	WorkerID     string
	Hostname     string
	Version      string
	Concurrency  int
	Capabilities []string
	Metadata     map[string]any
}

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	if in.Concurrency <= 0 {
		return apperr.Validatestion("concurrency must be > 0")
	}
	w := worker.Worker{WorkerID: in.WorkerID, Hostname: in.Hostname, Version: in.Version, Concurrency: in.Concurrency, Metadata: in.Metadata, Status: worker.StatusActive, RegisteredAt: s.clock.Now(), LastHeartbeatAt: s.clock.Now()}
	if err := s.repo.Register(ctx, w, in.Capabilities); err != nil {
		return apperr.Wrap(apperr.CodeInternal, "register worker", err)
	}
	return nil
}

type HeartbeatInput struct {
	WorkerID string
	Metrics  map[string]any
}

type HeartbeatOutput struct {
	CancelRequested []string
}

func (s *Service) Heartbeat(ctx context.Context, in HeartbeatInput) (HeartbeatOutput, error) {
	if err := s.repo.Heartbeat(ctx, in.WorkerID, s.clock.Now(), in.Metrics); err != nil {
		return HeartbeatOutput{}, apperr.Wrap(apperr.CodeInternal, "heartbeat", err)
	}
	return HeartbeatOutput{}, nil
}

type DrainInput struct {
	WorkerID string
}

func (s *Service) Drain(ctx context.Context, in DrainInput) error {
	if err := s.repo.SetStatus(ctx, in.WorkerID, worker.StatusDraining); err != nil {
		return apperr.Wrap(apperr.CodeInternal, "drain worker", err)
	}
	return nil
}

type GetInput struct {
	WorkerID string
}

func (s *Service) Get(ctx context.Context, in GetInput) (worker.Worker, error) {
	w, err := s.repo.Get(ctx, in.WorkerID)
	if err != nil {
		return worker.Worker{}, apperr.Wrap(apperr.CodeNotFound, "worker", err)
	}
	return w, nil
}

type ListInput struct{}

func (s *Service) List(ctx context.Context) ([]worker.Worker, error) {
	ws, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "list workers", err)
	}
	return ws, nil
}
