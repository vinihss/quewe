package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
	"github.com/vinihss/quewe/services/quewe/internal/adapters/outbound/memory"
	"github.com/vinihss/quewe/services/quewe/internal/adapters/outbound/postgres"
	appleasing "github.com/vinihss/quewe/services/quewe/internal/application/leasing"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/clock"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/config"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/id"
)

func buildDependencies(ctx context.Context, cfg config.Config, log *slog.Logger) (Dependencies, func(), error) {
	deps := Dependencies{Config: cfg, Log: log, Clock: clock.System{}, IDs: id.Random{}}
	cleanup := func() {}
	if cfg.DBEnabled {
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return Dependencies{}, cleanup, fmt.Errorf("pgxpool: %w", err)
		}
		cleanup = func() { pool.Close() }
		deps.Pool = pool
		deps.Tasks = postgres.NewTaskRepository(pool)
		deps.Executions = postgres.NewExecutionRepository(pool)
		deps.Queue = postgres.NewQueueRepository(pool)
		deps.Workers = postgres.NewWorkerRepository(pool)
		deps.Groups = postgres.NewGroupRepository(pool)
		deps.Ready = func(ctx context.Context) error { return pool.Ping(ctx) }
		deps.Leasing = appleasing.NewService(deps.Queue, deps.Executions, deps.Clock)
	} else {
		deps.Tasks = memory.NewTaskRepository()
		deps.Ready = func(context.Context) error { return nil }
	}
	if cfg.AllowDebugAuthStub() {
		deps.Auth = httpadapter.NewDebugStubAuthResolver()
		log.Warn("debug auth stub enabled")
	} else {
		deps.Auth = httpadapter.NewFailClosedAuthResolver()
	}
	return deps, cleanup, nil
}

func BuildDependencies(ctx context.Context, cfg config.Config, log *slog.Logger) (Dependencies, func(), error) {
	return buildDependencies(ctx, cfg, log)
}
