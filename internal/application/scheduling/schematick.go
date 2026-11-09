package scheduling

import (
	"context"
	"fmt"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	"github.com/vinihss/quewe/services/quewe/internal/ports/id"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
	"github.com/vinihss/quewe/services/quewe/internal/ports/schedule"
	"github.com/vinihss/quewe/services/quewe/internal/ports/tx"
)

const misfireWindow = 300 * time.Second

type Deps struct {
	Tasks  repositories.TaskRepository
	Execs  repositories.ExecutionRepository
	Scheds repositories.ScheduleRepository
	Calc   schedule.NextRunCalculator
	Tx     tx.Transactor
	Clock  clock.Clock
	IDGen  id.Generator
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) SchemaTick(ctx context.Context, now time.Time) error {
	due, err := s.d.Scheds.FindDue(ctx, now)
	if err != nil {
		return fmt.Errorf("find due: %w", err)
	}
	for _, tsk := range due {
		if err := s.processOne(ctx, tsk, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processOne(ctx context.Context, tsk task.Definition, now time.Time) error {
	return s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		sched, err := s.d.Scheds.GetSchedule(ctx, tsk.ID)
		if err != nil {
			return err
		}
		if sched.NextRunAt == nil {
			return nil
		}
		nextRun := *sched.NextRunAt

		// misfire handling
		late := now.Sub(nextRun) > misfireWindow
		if late && sched.MisfirePolicy == task.MisfireSkip {
			// skip: advance without creating
			nr, err := s.d.Calc.NextRun(ctx, sched.Expression, sched.Timezone, now)
			if err != nil {
				return err
			}
			lr := now
			return s.d.Scheds.AdvanceSchedule(ctx, tsk.ID, &lr, &nr)
		}
		if late && sched.MisfirePolicy == task.MisfireRunOnce {
			// recovery: create one execution with scheduled_for = original nextRun (delayed), then advance
			exec := execution.Execution{
				ID:           s.d.IDGen.New(),
				TaskID:       tsk.ID,
				Status:       execution.StatusQueued,
				Attempt:      1,
				MaxAttempts:  tsk.ExecutionPolicy.MaxAttempts(),
				ScheduledFor: nextRun,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := s.d.Execs.Create(ctx, exec); err != nil && err != repositories.ErrConflict {
				return fmt.Errorf("create recovery execution: %w", err)
			}
		}

		// create execution
		exec := execution.Execution{
			ID:           s.d.IDGen.New(),
			TaskID:       tsk.ID,
			Status:       execution.StatusQueued,
			Attempt:      1,
			MaxAttempts:  tsk.ExecutionPolicy.MaxAttempts(),
			ScheduledFor: nextRun,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.d.Execs.Create(ctx, exec); err != nil {
			if err == repositories.ErrConflict {
				// duplicate: skip
			} else {
				return fmt.Errorf("create execution: %w", err)
			}
		}

		// advance schedule
		nr, err := s.d.Calc.NextRun(ctx, sched.Expression, sched.Timezone, now)
		if err != nil {
			// for once schedules, no next; set nil
			if sched.Type == task.ScheduleOnce {
				lr := now
				return s.d.Scheds.AdvanceSchedule(ctx, tsk.ID, &lr, nil)
			}
			return err
		}
		lr := now
		return s.d.Scheds.AdvanceSchedule(ctx, tsk.ID, &lr, &nr)
	})
}
