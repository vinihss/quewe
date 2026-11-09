// Package app is the composition root and lifecycle owner.
//
// Guide rule (F4): wiring happens here, once, explicitly.
// No DI framework, no init(), no mutable global variables.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
	"github.com/vinihss/quewe/services/quewe/internal/application/leasing"
	"github.com/vinihss/quewe/services/quewe/internal/infrastructure/config"
)

// App owns the process resources.
type App struct {
	cfg            config.Config
	log            *slog.Logger
	server         *http.Server
	cleanup        func()
	leasingService *leasing.Service
}

// New wires dependencies, features and the HTTP server.
func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	deps, cleanup, err := buildDependencies(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	api := http.NewServeMux()
	if err := registerFeatures(api, deps); err != nil {
		cleanup()
		return nil, fmt.Errorf("register features: %w", err)
	}

	return &App{
		cfg:            cfg,
		log:            log,
		leasingService: deps.Leasing,
		cleanup:        cleanup,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           httpadapter.Mount(api, deps.Ready),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}, nil
}

// Run serves until the context is cancelled or the server fails.
//
// Guide rule (F5): orderly shutdown — stop accepting connections, drain
// requests, and only then close resources. Background loops follow the same
// contract, each with its own ctx and metrics.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	// Start the HTTP server
	go func() {
		a.log.Info("http server listening", "addr", a.cfg.HTTPAddr)
		errCh <- a.server.ListenAndServe()
	}()

	// Start the reaper loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				a.log.Info("reaper loop shutting down")
				return
			case <-ticker.C:
				a.log.Debug("reaper loop tick", "timestamp", time.Now().Format(time.RFC3339))
				if err := a.leasingService.Reap(ctx, time.Now()); err != nil {
					a.log.Error("reaper loop error", "error", err)
				} else {
					a.log.Debug("reaper loop completed")
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		a.log.Info("shutdown requested")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// Close releases resources that are not owned by the HTTP server.
func (a *App) Close() error {
	a.cleanup()
	return nil
}
