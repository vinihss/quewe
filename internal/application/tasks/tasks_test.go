package tasks_test

import (
	"context"
	"encoding/base64"
	"sort"
	"testing"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/application/tasks"
	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
	portschedule "github.com/vinihss/quewe/services/quewe/internal/ports/schedule"
)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

type fixedIDs struct {
	next []string
	i    int
}

func (f *fixedIDs) New() string {
	if f.i >= len(f.next) {
		return "id-overflow"
	}
	id := f.next[f.i]
	f.i++
	return id
}

type fakeRepo struct {
	items []task.Definition
}

func (r *fakeRepo) Create(_ context.Context, def task.Definition) error {
	for _, it := range r.items {
		if it.OwnerID == def.OwnerID && it.Name == def.Name {
			return repositories.ErrConflict
		}
	}
	r.items = append(r.items, def)
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, ownerID, id string) (task.Definition, error) {
	for _, it := range r.items {
		if it.ID == id && it.OwnerID == ownerID {
			return it, nil
		}
	}
	return task.Definition{}, repositories.ErrNotFound
}

func (r *fakeRepo) GetByTaskID(_ context.Context, taskID string) (task.Definition, error) {
	for _, it := range r.items {
		if it.ID == taskID {
			return it, nil
		}
	}
	return task.Definition{}, repositories.ErrNotFound
}

func (r *fakeRepo) List(_ context.Context, f repositories.TaskListFilter) (repositories.TaskPage, error) {
	var filtered []task.Definition
	for _, it := range r.items {
		if it.OwnerID != f.OwnerID {
			continue
		}
		if f.Enabled != nil && it.Enabled != *f.Enabled {
			continue
		}
		if f.Type != "" && it.Type != f.Type {
			continue
		}
		filtered = append(filtered, it)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	start := 0
	if f.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err == nil {
			lastID := string(b)
			for i, it := range filtered {
				if it.ID == lastID {
					start = i + 1
					break
				}
			}
		}
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + f.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := repositories.TaskPage{Items: filtered[start:end]}
	if end < len(filtered) && len(page.Items) > 0 {
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(page.Items[len(page.Items)-1].ID))
	}
	return page, nil
}

type notSupportedCron struct{}

func (notSupportedCron) NextRun(context.Context, string, string, time.Time) (time.Time, error) {
	return time.Time{}, portschedule.ErrNotSupported
}

type fixedCron struct{ next time.Time }

func (f fixedCron) NextRun(context.Context, string, string, time.Time) (time.Time, error) {
	return f.next, nil
}

func baseInput(owner string) tasks.CreateTaskInput {
	enabled := true
	return tasks.CreateTaskInput{
		OwnerID:     owner,
		Name:        "daily-arch-update",
		Type:        "system.command",
		Description: "update",
		Payload:     []byte(`{"op":"update"}`),
		Target:      tasks.TargetInput{Capability: "system.maintenance"},
		ExecutionPolicy: tasks.ExecutionPolicyInput{
			TimeoutSeconds: 300,
			MaxRetries:     2,
			Priority:       5,
		},
		Schedule: tasks.ScheduleInput{Type: "manual"},
		Enabled:  &enabled,
	}
}

func TestCreateTaskOKManual(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	svc := tasks.NewService(&fakeRepo{}, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, nil)
	def, err := svc.CreateTask(context.Background(), baseInput("owner-a"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if def.ID != "task-1" || def.OwnerID != "owner-a" {
		t.Fatalf("unexpected def: %+v", def)
	}
	if !def.CreatedAt.Equal(now) || !def.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps: got %v/%v", def.CreatedAt, def.UpdatedAt)
	}
	if def.Schedule.NextRunAt != nil {
		t.Fatalf("manual next_run_at should be nil")
	}
	if def.Schedule.MisfirePolicy != task.MisfireRunOnce {
		t.Fatalf("misfire default: %q", def.Schedule.MisfirePolicy)
	}
}

func TestCreateTaskOKOnce(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	runAt := time.Date(2026, 11, 6, 4, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	in := baseInput("owner-a")
	in.Name = "once-task"
	in.Schedule = tasks.ScheduleInput{Type: "once", RunAt: &runAt}
	svc := tasks.NewService(&fakeRepo{}, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, nil)
	def, err := svc.CreateTask(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if def.Schedule.NextRunAt == nil || !def.Schedule.NextRunAt.Equal(runAt.UTC()) {
		t.Fatalf("next_run_at: %v", def.Schedule.NextRunAt)
	}
}

func TestCreateTaskCronNotSupported(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	in := baseInput("owner-a")
	in.Name = "cron-task"
	in.Schedule = tasks.ScheduleInput{Type: "cron", Expression: "0 4 * * *", Timezone: "America/Sao_Paulo"}
	svc := tasks.NewService(&fakeRepo{}, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, notSupportedCron{})
	_, err := svc.CreateTask(context.Background(), in)
	if !apperr.IsCode(err, apperr.CodeNotSupported) {
		t.Fatalf("want not_supported, got %v", err)
	}
}

func TestCreateTaskCronWithCalculator(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	next := time.Date(2026, 11, 6, 7, 0, 0, 0, time.UTC)
	in := baseInput("owner-a")
	in.Name = "cron-task"
	in.Schedule = tasks.ScheduleInput{Type: "cron", Expression: "0 4 * * *", Timezone: "America/Sao_Paulo"}
	svc := tasks.NewService(&fakeRepo{}, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, fixedCron{next: next})
	def, err := svc.CreateTask(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if def.Schedule.NextRunAt == nil || !def.Schedule.NextRunAt.Equal(next) {
		t.Fatalf("next_run_at: %v", def.Schedule.NextRunAt)
	}
}

func TestCreateTaskValidatestionErrors(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	svc := tasks.NewService(&fakeRepo{}, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, nil)

	tests := []struct {
		name string
		mut  func(*tasks.CreateTaskInput)
	}{
		{name: "invalid name", mut: func(in *tasks.CreateTaskInput) { in.Name = "Bad_Name" }},
		{name: "priority invalid", mut: func(in *tasks.CreateTaskInput) { in.ExecutionPolicy.Priority = 10 }},
		{name: "timeout invalid", mut: func(in *tasks.CreateTaskInput) { in.ExecutionPolicy.TimeoutSeconds = 0 }},
		{name: "max retries invalid", mut: func(in *tasks.CreateTaskInput) { in.ExecutionPolicy.MaxRetries = -1 }},
		{name: "missing capability", mut: func(in *tasks.CreateTaskInput) { in.Target.Capability = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput("owner-a")
			tt.mut(&in)
			_, err := svc.CreateTask(context.Background(), in)
			if !apperr.IsCode(err, apperr.CodeValidatestion) {
				t.Fatalf("want validatestion_error, got %v", err)
			}
		})
	}
}

func TestCreateTaskConflict(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	svc := tasks.NewService(repo, fixedClock{t: now}, &fixedIDs{next: []string{"task-1", "task-2"}}, nil)
	if _, err := svc.CreateTask(context.Background(), baseInput("owner-a")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.CreateTask(context.Background(), baseInput("owner-a"))
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	// same name, different owner is allowed
	in := baseInput("owner-b")
	if _, err := svc.CreateTask(context.Background(), in); err != nil {
		t.Fatalf("different owner should succeed: %v", err)
	}
}

func TestGetTaskNotFoundAndOwnerIsolation(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	svc := tasks.NewService(repo, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, nil)
	created, err := svc.CreateTask(context.Background(), baseInput("owner-a"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.GetTask(context.Background(), tasks.GetTaskInput{OwnerID: "owner-a", ID: "missing"})
	if !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("want not_found, got %v", err)
	}
	_, err = svc.GetTask(context.Background(), tasks.GetTaskInput{OwnerID: "owner-b", ID: created.ID})
	if !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("owner isolation should return not_found, got %v", err)
	}
	got, err := svc.GetTask(context.Background(), tasks.GetTaskInput{OwnerID: "owner-a", ID: created.ID})
	if err != nil || got.ID != created.ID {
		t.Fatalf("get ok: got=%+v err=%v", got, err)
	}
}

func TestListTasksPaginationAndFilters(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	svc := tasks.NewService(repo, fixedClock{t: base}, &fixedIDs{next: []string{"x"}}, nil)

	repo.items = []task.Definition{
		{ID: "t1", OwnerID: "o1", Name: "a-one", Type: "system.command", Enabled: true, CreatedAt: base.Add(1 * time.Minute), Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
		{ID: "t2", OwnerID: "o1", Name: "b-two", Type: "research.web", Enabled: true, CreatedAt: base.Add(2 * time.Minute), Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
		{ID: "t3", OwnerID: "o1", Name: "c-three", Type: "system.command", Enabled: false, CreatedAt: base.Add(3 * time.Minute), Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
		{ID: "t4", OwnerID: "o2", Name: "d-other", Type: "system.command", Enabled: true, CreatedAt: base.Add(4 * time.Minute), Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
	}

	page1, err := svc.ListTasks(ctx, tasks.ListTasksInput{OwnerID: "o1", Limit: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page1.Items) != 2 || page1.Items[0].ID != "t3" || page1.Items[1].ID != "t2" {
		t.Fatalf("page1: %+v", idsOf(page1.Items))
	}
	if page1.NextCursor == "" {
		t.Fatal("expected next cursor")
	}
	page2, err := svc.ListTasks(ctx, tasks.ListTasksInput{OwnerID: "o1", Limit: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("list2: %v", err)
	}
	if len(page2.Items) != 1 || page2.Items[0].ID != "t1" {
		t.Fatalf("page2: %+v", idsOf(page2.Items))
	}

	enabledTrue := true
	filtered, err := svc.ListTasks(ctx, tasks.ListTasksInput{OwnerID: "o1", Enabled: &enabledTrue, Type: "system.command"})
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].ID != "t1" {
		t.Fatalf("filtered: %+v", idsOf(filtered.Items))
	}
}

func idsOf(items []task.Definition) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}
