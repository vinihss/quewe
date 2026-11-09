//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
	"github.com/vinihss/quewe/services/quewe/internal/adapters/outbound/cron"
	"github.com/vinihss/quewe/services/quewe/internal/adapters/outbound/postgres"
	"github.com/vinihss/quewe/services/quewe/internal/application/scheduling"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/app"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/config"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/logging"
)

func dbURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return os.Getenv("DATABASE_URL")
}

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := dbURL()
	if url == "" {
		t.Skip("TEST_DATABASE_URL/DATABASE_URL ausente; pulando teste de integração")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	for _, f := range []string{"0001_init.up.sql", "0002_broker.up.sql"} {
		data, err := os.ReadFile("../../migrations/" + f)
		if err != nil {
			continue
		}
		_, _ = pool.Exec(context.Background(), string(data))
	}
	t.Cleanup(func() { pool.Close() })
	_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE tasks, executions, queue_messages, workers, schedules CASCADE")
	return pool
}

func testApp(t *testing.T) (baseURL string, sched *scheduling.Service, cleanup func()) {
	t.Helper()
	os.Setenv("APP_ENV", "dev")
	os.Setenv("DEBUG_AUTH_STUB", "true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DBEnabled {
		t.Skip("DATABASE_URL ausente; pulando")
	}
	logger := logging.New("quewe", "debug")
	deps, cleanupDeps, err := app.BuildDependencies(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	api := http.NewServeMux()
	if err := app.RegisterFeatures(api, deps); err != nil {
		cleanupDeps()
		t.Fatal(err)
	}
	handler := httpadapter.Mount(api, deps.Ready)
	srv := httptest.NewServer(handler)
	sched = scheduling.New(scheduling.Deps{Tasks: deps.Tasks, Execs: deps.Executions, Scheds: postgres.NewScheduleRepository(deps.Pool), Calc: cron.New(), Tx: postgres.NewTransactor(deps.Pool), Clock: deps.Clock, IDGen: deps.IDs})
	cleanup = func() { srv.Close(); cleanupDeps() }
	return srv.URL, sched, cleanup
}
