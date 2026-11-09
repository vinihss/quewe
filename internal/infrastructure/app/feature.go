package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vinihss/quewe/services/quewe/internal/application/leasing"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/clock"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/config"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/id"
	portauth "github.com/vinihss/quewe/services/quewe/internal/ports/auth"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
	portschedule "github.com/vinihss/quewe/services/quewe/internal/ports/schedule"
)

// Dependencies is the shared service context handed to every feature.
//
// Guide rule (F4): features receive only what they need as parameters.
// This struct grows by mode (full adds DB) and by shared port,
// never by global variable.
type Dependencies struct {
	Config config.Config
	Log    *slog.Logger
	Clock  clock.System
	IDs    id.Random

	Tasks      repositories.TaskRepository
	Executions repositories.ExecutionRepository
	Queue      repositories.QueueRepository
	Workers    repositories.WorkerRepository
	Groups     repositories.GroupRepository
	Pool       *pgxpool.Pool
	Ready      func(context.Context) error
	Cron       portschedule.NextRunCalculator // nil until ADR-003 calculator wired
	Auth       portauth.Resolver
	Leasing    *leasing.Service
}

// Feature registers one feature on the versioned API mux.
type Feature func(api *http.ServeMux, deps Dependencies) error
