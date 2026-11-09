// Package logging builds the structured logger.
//
// Guide rule (F3): log é JSON estruturado (log/slog), com nível validatesdo na
// configuration. Nunca fmt.Println para diagnóstico operacional.
//
// Field conventions (AGENTS.md §5): service é atributo base fixo; trace_id,
// task_id, execution_id, worker_id e attempt devem ser propagados quando
// disponíveis no contexto da operação. Não registrar segredos nem payloads
// sensíveis.
package logging

import (
	"log/slog"
	"os"
)

// New builds a JSON logger for the given service and level.
func New(service, level string) *slog.Logger {
	options := &slog.HandlerOptions{Level: parseLevel(level)}
	handler := slog.NewJSONHandler(os.Stdout, options)
	return slog.New(handler).With(slog.String("service", service))
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
