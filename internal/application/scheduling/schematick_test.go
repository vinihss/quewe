package scheduling

import (
	"context"
	"errors"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	"github.com/vinihss/quewe/services/quewe/internal/ports/id"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
	"github.com/vinihss/quewe/services/quewe/internal/ports/schedule"
	"github.com/vinihss/quewe/services/quewe/internal/ports/tx"
)

// ErrNotSupported is the schedule next-run error.
var ErrNotSupported = errors.New("schedule: cron next-run calculation not supported")

// ---- fakes ----

type fakeExecRepo struct {
	createErr error
	creates   []execution.Execution
}

func (f *fakeExecRepo) Create(ctx context.Context, e execution.Execution) error {
	f.creates = append(f.creates, e)
	return f.createErr
}
func (f *fakeExecRepo) GetByID(context.Context, string, string) (execution.Execution, error) {
	return execution.Execution{}, repositories.ErrNotFound
}
func (f *fakeExecRepo) List(context.Context, repositories.ExecutionListFilter) (repositories.ExecutionPage, error) {
	return repositories.ExecutionPage{}, nil
}
func (f *fakeExecRepo) GetByExecutionID(context.Context, string) (execution.Execution, error) {
	return execution.Execution{}, nil
}
func (f *fakeExecRepo) UpdateStatus(context.Context, string, execution.Status, execution.Status) error {
	return nil
}
func (f *fakeExecRepo) EnqueueRequeue(context.Context, string, time.Time) error {
	return nil
}
func (f *fakeExecRepo) GetRedriveCount(context.Context, string) (int, error) {
	return 0, nil
}
func (f *fakeExecRepo) SetRedriveCount(context.Context, string, int) error {
	return nil
}
func (f *fakeExecRepo) SetResult(context.Context, string, *string, *string, *string) error {
	return nil
}
func (f *fakeExecRepo) SetWorker(context.Context, string, string, int, int) error {
	return nil
}

type fakeTx struct{}

func (fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeClock struct{}

func (fakeClock) Now() time.Time {
	return time.Now()
}

type fakeCalc struct{}

func (fakeCalc) NextRun(ctx context.Context, expression, timezone string, after time.Time) (time.Time, error) {
	return time.Time{}, ErrNotSupported
}

type fakeIDGen struct{}

func (fakeIDGen) New() string {
	return "test-id"
}

type fakeTaskRepo struct{}

func (f *fakeTaskRepo) Create(ctx context.Context, def task.Definition) error { return nil }
func (f *fakeTaskRepo) GetByID(context.Context, string, string) (task.Definition, error) {
	return task.Definition{}, nil
}
func (f *fakeTaskRepo) List(context.Context, repositories.TaskListFilter) (repositories.TaskPage, error) {
	return repositories.TaskPage{}, nil
}
func (f *fakeTaskRepo) GetByTaskID(context.Context, string) (task.Definition, error) {
	return task.Definition{}, nil
}

type fakeSchedRepo struct{}

func (f *fakeSchedRepo) FindDue(context.Context, time.Time) ([]task.Definition, error) {
	return nil, nil
}
func (f *fakeSchedRepo) AdvanceSchedule(context.Context, string, *time.Time, *time.Time) error {
	return nil
}
func (f *fakeSchedRepo) GetSchedule(context.Context, string) (task.Schedule, error) {
	return task.Schedule{}, nil
}
func (f *fakeSchedRepo) Create(ctx context.Context, s task.Schedule) error { return nil }

var (
	_ repositories.TaskRepository      = (*fakeTaskRepo)(nil)
	_ repositories.ExecutionRepository = (*fakeExecRepo)(nil)
	_ repositories.ScheduleRepository  = (*fakeSchedRepo)(nil)
	_ tx.Transactor                    = fakeTx{}
	_ clock.Clock                      = fakeClock{}
	_ id.Generator                     = (*fakeIDGen)(nil)
	_ schedule.NextRunCalculator       = (*fakeCalc)(nil)
)
