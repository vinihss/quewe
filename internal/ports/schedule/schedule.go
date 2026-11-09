package schedule

import (
	"context"
	"errors"
	"time"
)

var ErrNotSupported = errors.New("schedule: cron next-run calculation not supported")

// NextRunCalculator computes the next occurrence for a cron schedule.
// ADR-003 requires deterministic, DST-aware calculation; WP2.1 defines the port
// without adding a CRON library dependency.
type NextRunCalculator interface {
	NextRun(ctx context.Context, expression, timezone string, after time.Time) (time.Time, error)
}
