package tasks

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	portclock "github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	portid "github.com/vinihss/quewe/services/quewe/internal/ports/id"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
	portschedule "github.com/vinihss/quewe/services/quewe/internal/ports/schedule"
)

type Service struct {
	repo  repositories.TaskRepository
	clock portclock.Clock
	ids   portid.Generator
	cron  portschedule.NextRunCalculator
}

func NewService(repo repositories.TaskRepository, clock portclock.Clock, ids portid.Generator, cron portschedule.NextRunCalculator) *Service {
	return &Service{repo: repo, clock: clock, ids: ids, cron: cron}
}

type TargetInput struct {
	Capability string
	WorkerID   *string
}

type ExecutionPolicyInput struct {
	TimeoutSeconds int
	MaxRetries     int
	Priority       int
}

type ScheduleInput struct {
	Type          string
	Expression    string
	Timezone      string
	RunAt         *time.Time
	MisfirePolicy string
}

type CreateTaskInput struct {
	OwnerID         string
	Name            string
	Type            string
	Description     string
	Payload         []byte
	Target          TargetInput
	ExecutionPolicy ExecutionPolicyInput
	Schedule        ScheduleInput
	Enabled         *bool
}

func (s *Service) CreateTask(ctx context.Context, in CreateTaskInput) (task.Definition, error) {
	if strings.TrimSpace(in.OwnerID) == "" {
		return task.Definition{}, apperr.Validatestion("owner_id is required")
	}
	if err := task.ValidatesteName(in.Name); err != nil {
		return task.Definition{}, apperr.Validatestion(err.Error())
	}
	if strings.TrimSpace(in.Type) == "" {
		return task.Definition{}, apperr.Validatestion("task type is required")
	}

	policy := task.ExecutionPolicy{
		TimeoutSeconds: in.ExecutionPolicy.TimeoutSeconds,
		MaxRetries:     in.ExecutionPolicy.MaxRetries,
		Priority:       in.ExecutionPolicy.Priority,
	}
	if err := policy.Validateste(); err != nil {
		return task.Definition{}, apperr.Validatestion(err.Error())
	}
	target := task.Target{Capability: in.Target.Capability, WorkerID: in.Target.WorkerID}
	if err := target.Validateste(); err != nil {
		return task.Definition{}, apperr.Validatestion(err.Error())
	}

	sched, err := s.buildSchedule(ctx, in.Schedule)
	if err != nil {
		return task.Definition{}, err
	}

	now := s.clock.Now().UTC()
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	def := task.Definition{
		ID:              s.ids.New(),
		OwnerID:         in.OwnerID,
		Name:            in.Name,
		Type:            in.Type,
		Description:     in.Description,
		Payload:         append([]byte(nil), in.Payload...),
		Target:          target,
		ExecutionPolicy: policy,
		Schedule:        sched,
		Enabled:         enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, def); err != nil {
		if errors.Is(err, repositories.ErrConflict) {
			return task.Definition{}, apperr.Conflict("task name already exists for owner")
		}
		return task.Definition{}, apperr.Wrap(apperr.CodeInternal, "create task", err)
	}
	return def, nil
}

func (s *Service) buildSchedule(ctx context.Context, in ScheduleInput) (task.Schedule, error) {
	st := task.ScheduleType(in.Type)
	mp := task.MisfirePolicy(in.MisfirePolicy)
	if mp == "" {
		mp = task.MisfireRunOnce
	}
	sched := task.Schedule{
		Type:          st,
		Expression:    in.Expression,
		Timezone:      in.Timezone,
		RunAt:         cloneTime(in.RunAt),
		MisfirePolicy: mp,
	}
	if err := sched.Validateste(); err != nil {
		return task.Schedule{}, apperr.Validatestion(err.Error())
	}

	switch st {
	case task.ScheduleManual:
		sched.NextRunAt = nil
	case task.ScheduleOnce:
		sched.NextRunAt = cloneTime(in.RunAt)
	case task.ScheduleCron:
		if s.cron == nil {
			return task.Schedule{}, apperr.NotSupported("cron next_run_at calculation is not configured; provide a schedule.NextRunCalculator (ADR-003)")
		}
		now := s.clock.Now().UTC()
		next, err := s.cron.NextRun(ctx, in.Expression, in.Timezone, now)
		if err != nil {
			if errors.Is(err, portschedule.ErrNotSupported) {
				return task.Schedule{}, apperr.NotSupported("cron next_run_at calculation is not supported by the configured calculator (ADR-003)")
			}
			return task.Schedule{}, apperr.Wrap(apperr.CodeValidatestion, "calculate cron next_run_at", err)
		}
		sched.NextRunAt = &next
	default:
		return task.Schedule{}, apperr.Validatestion("unsupported schedule type")
	}
	return sched, nil
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}

type GetTaskInput struct {
	OwnerID string
	ID      string
}

func (s *Service) GetTask(ctx context.Context, in GetTaskInput) (task.Definition, error) {
	if strings.TrimSpace(in.OwnerID) == "" {
		return task.Definition{}, apperr.Validatestion("owner_id is required")
	}
	if strings.TrimSpace(in.ID) == "" {
		return task.Definition{}, apperr.Validatestion("task id is required")
	}
	def, err := s.repo.GetByID(ctx, in.OwnerID, in.ID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return task.Definition{}, apperr.NotFound("task not found")
		}
		return task.Definition{}, apperr.Wrap(apperr.CodeInternal, "get task", err)
	}
	return def, nil
}

type ListTasksInput struct {
	OwnerID string
	Enabled *bool
	Type    string
	Limit   int
	Cursor  string
}

func (s *Service) ListTasks(ctx context.Context, in ListTasksInput) (repositories.TaskPage, error) {
	if strings.TrimSpace(in.OwnerID) == "" {
		return repositories.TaskPage{}, apperr.Validatestion("owner_id is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	page, err := s.repo.List(ctx, repositories.TaskListFilter{
		OwnerID: in.OwnerID,
		Enabled: in.Enabled,
		Type:    in.Type,
		Limit:   limit,
		Cursor:  in.Cursor,
	})
	if err != nil {
		return repositories.TaskPage{}, apperr.Wrap(apperr.CodeInternal, "list tasks", err)
	}
	return page, nil
}
