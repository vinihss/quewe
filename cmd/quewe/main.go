// Command quewe is the process entrypoint.
//
// Guide rule (F1): main.go only loads configuration, wires the application,
// handles signals, and delegates the lifecycle. No business logic here.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/app"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/config"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New("quewe", cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer application.Close()

	return application.Run(ctx)
}
