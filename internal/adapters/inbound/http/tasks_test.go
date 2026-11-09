package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	apptasks "github.com/vinihss/quewe/services/quewe/internal/application/tasks"
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
		return fmt.Sprintf("id-%d", f.i)
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
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := repositories.TaskPage{Items: append([]task.Definition(nil), filtered[start:end]...)}
	if end < len(filtered) && len(page.Items) > 0 {
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(page.Items[len(page.Items)-1].ID))
	}
	return page, nil
}

type notSupportedCron struct{}

func (notSupportedCron) NextRun(context.Context, string, string, time.Time) (time.Time, error) {
	return time.Time{}, portschedule.ErrNotSupported
}

func debugServer(t *testing.T, repo *fakeRepo, cron portschedule.NextRunCalculator) nethttp.Handler {
	t.Helper()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	svc := apptasks.NewService(repo, fixedClock{t: now}, &fixedIDs{next: []string{"task-1", "task-2", "task-3", "task-4", "task-5"}}, cron)
	h := NewTasksHandler(svc, NewDebugStubAuthResolver())
	api := nethttp.NewServeMux()
	h.Register(api)
	return Mount(api, func(context.Context) error { return nil })
}

func failClosedServer(t *testing.T, repo *fakeRepo) nethttp.Handler {
	t.Helper()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	svc := apptasks.NewService(repo, fixedClock{t: now}, &fixedIDs{next: []string{"task-1"}}, nil)
	h := NewTasksHandler(svc, NewFailClosedAuthResolver())
	api := nethttp.NewServeMux()
	h.Register(api)
	return Mount(api, func(context.Context) error { return nil })
}

func doJSON(t *testing.T, h nethttp.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var req *nethttp.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, rec.Body.String())
	}
	return out
}

func TestCreateTaskCreated(t *testing.T) {
	h := debugServer(t, &fakeRepo{}, nil)
	rec := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", `{
		"name":"daily-arch-update",
		"type":"system.command",
		"description":"update",
		"payload":{"op":"update"},
		"target":{"capability":"system.maintenance"},
		"execution_policy":{"timeout_seconds":300,"max_retries":2,"priority":5},
		"schedule":{"type":"manual"}
	}`, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if rec.Code != nethttp.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["id"] != "task-1" || resp["owner_id"] != "2eb2bc3a-0876-33eb-2961-2a3127b02762" {
		t.Fatalf("resp=%v", resp)
	}
	if resp["name"] != "daily-arch-update" {
		t.Fatalf("name=%v", resp["name"])
	}
}

func TestCreateTaskValidatestion400MalformedJSON(t *testing.T) {
	h := debugServer(t, &fakeRepo{}, nil)
	rec := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", `{"name":`, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if rec.Code != nethttp.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	errObj := decodeErr(t, rec)["error"].(map[string]any)
	if errObj["code"] != "validatestion_error" {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestCreateTaskValidatestion422Domain(t *testing.T) {
	h := debugServer(t, &fakeRepo{}, nil)
	rec := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", `{
		"name":"Bad_Name",
		"type":"system.command",
		"target":{"capability":"system.maintenance"},
		"execution_policy":{"timeout_seconds":300,"max_retries":2,"priority":5},
		"schedule":{"type":"manual"}
	}`, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if rec.Code != nethttp.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	errObj := decodeErr(t, rec)["error"].(map[string]any)
	if errObj["code"] != "validatestion_error" {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestCreateTaskConflict409(t *testing.T) {
	repo := &fakeRepo{}
	h := debugServer(t, repo, nil)
	body := `{
		"name":"daily-arch-update",
		"type":"system.command",
		"target":{"capability":"system.maintenance"},
		"execution_policy":{"timeout_seconds":300,"max_retries":2,"priority":5},
		"schedule":{"type":"manual"}
	}`
	rec1 := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", body, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if rec1.Code != nethttp.StatusCreated {
		t.Fatalf("first status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", body, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if rec2.Code != nethttp.StatusConflict {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	errObj := decodeErr(t, rec2)["error"].(map[string]any)
	if errObj["code"] != "conflict" {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestGetTask200And404(t *testing.T) {
	h := debugServer(t, &fakeRepo{}, nil)
	create := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", `{
		"name":"daily-arch-update","type":"system.command",
		"target":{"capability":"system.maintenance"},
		"execution_policy":{"timeout_seconds":300,"max_retries":2,"priority":5},
		"schedule":{"type":"manual"}
	}`, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if create.Code != nethttp.StatusCreated {
		t.Fatalf("create status=%d", create.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(create.Body.Bytes(), &created)
	id := created["id"].(string)

	get := doJSON(t, h, nethttp.MethodGet, "/api/v1/tasks/"+id, "", map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if get.Code != nethttp.StatusOK {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
	miss := doJSON(t, h, nethttp.MethodGet, "/api/v1/tasks/00000000-0000-0000-0000-000000000000", "", map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if miss.Code != nethttp.StatusNotFound {
		t.Fatalf("miss status=%d body=%s", miss.Code, miss.Body.String())
	}
	errObj := decodeErr(t, miss)["error"].(map[string]any)
	if errObj["code"] != "not_found" {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestListTasks200PaginationFiltersOwnerIsolation(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{items: []task.Definition{
		{ID: "t1", OwnerID: "a473a8c1-ab29-e55e-40e1-4f2c8d0fe078", Name: "a-one", Type: "system.command", Enabled: true, CreatedAt: now.Add(1 * time.Minute), UpdatedAt: now, Target: task.Target{Capability: "c"}, ExecutionPolicy: task.ExecutionPolicy{TimeoutSeconds: 1, Priority: 1}, Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
		{ID: "t2", OwnerID: "a473a8c1-ab29-e55e-40e1-4f2c8d0fe078", Name: "b-two", Type: "research.web", Enabled: true, CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now, Target: task.Target{Capability: "c"}, ExecutionPolicy: task.ExecutionPolicy{TimeoutSeconds: 1, Priority: 1}, Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
		{ID: "t3", OwnerID: "a473a8c1-ab29-e55e-40e1-4f2c8d0fe078", Name: "c-three", Type: "system.command", Enabled: false, CreatedAt: now.Add(3 * time.Minute), UpdatedAt: now, Target: task.Target{Capability: "c"}, ExecutionPolicy: task.ExecutionPolicy{TimeoutSeconds: 1, Priority: 1}, Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
		{ID: "t4", OwnerID: "0f927dce-74ec-3475-c7a3-9299e5bffab2", Name: "d-other", Type: "system.command", Enabled: true, CreatedAt: now.Add(4 * time.Minute), UpdatedAt: now, Target: task.Target{Capability: "c"}, ExecutionPolicy: task.ExecutionPolicy{TimeoutSeconds: 1, Priority: 1}, Schedule: task.Schedule{Type: task.ScheduleManual, MisfirePolicy: task.MisfireRunOnce}},
	}}
	// fixed IDs irrelevant here; use enough
	svc := apptasks.NewService(repo, fixedClock{t: now}, &fixedIDs{next: []string{"x"}}, nil)
	h := NewTasksHandler(svc, NewDebugStubAuthResolver())
	api := nethttp.NewServeMux()
	h.Register(api)
	srv := Mount(api, func(context.Context) error { return nil })

	rec1 := doJSON(t, srv, nethttp.MethodGet, "/api/v1/tasks?limit=2", "", map[string]string{"X-Debug-Principal-Id": "a473a8c1-ab29-e55e-40e1-4f2c8d0fe078"})
	if rec1.Code != nethttp.StatusOK {
		t.Fatalf("status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	var page1 map[string]any
	_ = json.Unmarshal(rec1.Body.Bytes(), &page1)
	items1 := page1["items"].([]any)
	if len(items1) != 2 || items1[0].(map[string]any)["id"] != "t3" {
		t.Fatalf("page1=%v", page1)
	}
	cursor := page1["next_cursor"].(string)
	rec2 := doJSON(t, srv, nethttp.MethodGet, "/api/v1/tasks?limit=2&cursor="+cursor, "", map[string]string{"X-Debug-Principal-Id": "a473a8c1-ab29-e55e-40e1-4f2c8d0fe078"})
	var page2 map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &page2)
	items2 := page2["items"].([]any)
	if len(items2) != 1 || items2[0].(map[string]any)["id"] != "t1" {
		t.Fatalf("page2=%v", page2)
	}

	// owner isolation: o2 cannot see o1 tasks via their principal
	recO2 := doJSON(t, srv, nethttp.MethodGet, "/api/v1/tasks", "", map[string]string{"X-Debug-Principal-Id": "0f927dce-74ec-3475-c7a3-9299e5bffab2"})
	var pageO2 map[string]any
	_ = json.Unmarshal(recO2.Body.Bytes(), &pageO2)
	itemsO2 := pageO2["items"].([]any)
	if len(itemsO2) != 1 || itemsO2[0].(map[string]any)["id"] != "t4" {
		t.Fatalf("owner isolation page=%v", pageO2)
	}

	// filters
	recF := doJSON(t, srv, nethttp.MethodGet, "/api/v1/tasks?enabled=true&type=system.command", "", map[string]string{"X-Debug-Principal-Id": "a473a8c1-ab29-e55e-40e1-4f2c8d0fe078"})
	var pageF map[string]any
	_ = json.Unmarshal(recF.Body.Bytes(), &pageF)
	itemsF := pageF["items"].([]any)
	if len(itemsF) != 1 || itemsF[0].(map[string]any)["id"] != "t1" {
		t.Fatalf("filtered=%v", pageF)
	}
}

func TestAuthFailClosedWithoutDevStub(t *testing.T) {
	// No debug stub: fail-closed resolver is the safe default (wired when
	// APP_ENV!=dev || DEBUG_AUTH_STUB!=true). Must not bypass auth.
	h := failClosedServer(t, &fakeRepo{})

	t.Run("missing auth", func(t *testing.T) {
		rec := doJSON(t, h, nethttp.MethodGet, "/api/v1/tasks", "", nil)
		if rec.Code != nethttp.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		errObj := decodeErr(t, rec)["error"].(map[string]any)
		if errObj["code"] != "unauthorized" {
			t.Fatalf("code=%v", errObj["code"])
		}
	})

	t.Run("bearer present but resolver not configured", func(t *testing.T) {
		rec := doJSON(t, h, nethttp.MethodGet, "/api/v1/tasks", "", map[string]string{"Authorization": "Bearer token"})
		if rec.Code != nethttp.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		errObj := decodeErr(t, rec)["error"].(map[string]any)
		if errObj["code"] != "forbidden" {
			t.Fatalf("code=%v", errObj["code"])
		}
	})
}

func TestAuthDebugStubRequiresHeader(t *testing.T) {
	h := debugServer(t, &fakeRepo{}, nil)
	// Stub is only wired by app when config.AllowDebugAuthStub() is true
	// (APP_ENV=dev && DEBUG_AUTH_STUB=true). Here we test the resolver behavior
	// directly; wiring gate is covered by config tests.
	rec := doJSON(t, h, nethttp.MethodGet, "/api/v1/tasks", "", nil)
	if rec.Code != nethttp.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	ok := doJSON(t, h, nethttp.MethodGet, "/api/v1/tasks", "", map[string]string{"X-Debug-Principal-Id": "dev-user"})
	if ok.Code != nethttp.StatusOK {
		t.Fatalf("status=%d body=%s", ok.Code, ok.Body.String())
	}
}

func TestCronNotSupportedMapped422(t *testing.T) {
	h := debugServer(t, &fakeRepo{}, notSupportedCron{})
	rec := doJSON(t, h, nethttp.MethodPost, "/api/v1/tasks", `{
		"name":"cron-task","type":"system.command",
		"target":{"capability":"system.maintenance"},
		"execution_policy":{"timeout_seconds":300,"max_retries":2,"priority":5},
		"schedule":{"type":"cron","expression":"0 4 * * *","timezone":"America/Sao_Paulo"}
	}`, map[string]string{"X-Debug-Principal-Id": "owner-a"})
	if rec.Code != nethttp.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	errObj := decodeErr(t, rec)["error"].(map[string]any)
	if errObj["code"] != "validatestion_error" {
		t.Fatalf("code=%v", errObj["code"])
	}
	if !strings.Contains(fmt.Sprint(errObj["message"]), "cron") {
		t.Fatalf("message=%v", errObj["message"])
	}
}
