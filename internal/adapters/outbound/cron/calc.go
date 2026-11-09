package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/vinihss/quewe/services/quewe/internal/ports/schedule"
)

// Calculator implements ports/schedule.NextRunCalculator using robfig/cron/v3
// as parser only; occurrence math is cron's Schedule.Next.
type Calculator struct{}

func New() *Calculator { return &Calculator{} }

func (c *Calculator) NextRun(ctx context.Context, expression, timezone string, after time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("schedule: load location %q: %w", timezone, err)
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expression)
	if err != nil {
		// try 6-field (with seconds) per ADR-003
		parser6 := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sched, err = parser6.Parse(expression)
		if err != nil {
			return time.Time{}, fmt.Errorf("schedule: parse cron %q: %w", expression, err)
		}
	}
	// Evaluate in task timezone: convert after to loc, compute next, convert back to UTC.
	localAfter := after.In(loc)
	next := sched.Next(localAfter)
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("schedule: no next occurrence")
	}
	return next.UTC(), nil
}

var _ schedule.NextRunCalculator = (*Calculator)(nil)
