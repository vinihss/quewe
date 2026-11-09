package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/dto"
	apptasks "github.com/vinihss/quewe/services/quewe/internal/application/tasks"
	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/auth"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

// TaskService is the application port consumed by the HTTP adapter.
type TaskService interface {
	CreateTask(ctx context.Context, in apptasks.CreateTaskInput) (task.Definition, error)
	GetTask(ctx context.Context, in apptasks.GetTaskInput) (task.Definition, error)
	ListTasks(ctx context.Context, in apptasks.ListTasksInput) (repositories.TaskPage, error)
}

// TasksHandler handles task CRUD HTTP endpoints.
type TasksHandler struct {
	svc  TaskService
	auth auth.Resolver
}

// NewTasksHandler creates a handler. Auth resolver must be non-nil.
func NewTasksHandler(svc TaskService, resolver auth.Resolver) *TasksHandler {
	return &TasksHandler{svc: svc, auth: resolver}
}

// Register mounts task routes on the versioned API mux (already stripped of /api/v1).
func (h *TasksHandler) Register(api *http.ServeMux) {
	api.HandleFunc("POST /tasks", h.createTask)
	api.HandleFunc("GET /tasks", h.listTasks)
	api.HandleFunc("GET /tasks/{id}", h.getTask)
}

func (h *TasksHandler) principal(r *http.Request) (auth.Principal, error) {
	ctx := contextWithRequest(r.Context(), r)
	p, err := h.auth.Resolve(ctx)
	if err != nil {
		return auth.Principal{}, err
	}
	return p, nil
}

func (h *TasksHandler) createTask(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	// Idempotency-Key is accepted (recommended) but not guaranteed without
	// persistence in WP2.3 (ADR-014). Do not claim idempotent deduplication.
	_ = strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	var req dto.CreateTaskRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	if err := validatesteCreateRequest(req); err != nil {
		writeError(w, r, err)
		return
	}

	def, err := h.svc.CreateTask(r.Context(), apptasks.CreateTaskInput{
		OwnerID:     p.ID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Payload:     bytesOrNil(req.Payload),
		Target: apptasks.TargetInput{
			Capability: req.Target.Capability,
			WorkerID:   req.Target.WorkerID,
		},
		ExecutionPolicy: apptasks.ExecutionPolicyInput{
			TimeoutSeconds: req.ExecutionPolicy.TimeoutSeconds,
			MaxRetries:     req.ExecutionPolicy.MaxRetries,
			Priority:       req.ExecutionPolicy.Priority,
		},
		Schedule: apptasks.ScheduleInput{
			Type:          req.Schedule.Type,
			Expression:    req.Schedule.Expression,
			Timezone:      req.Schedule.Timezone,
			RunAt:         req.Schedule.RunAt,
			MisfirePolicy: req.Schedule.MisfirePolicy,
		},
		Enabled: req.Enabled,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, taskToResponse(def))
}

func (h *TasksHandler) getTask(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, r, fmt.Errorf("%w: task id is required", errInvalidBody))
		return
	}
	def, err := h.svc.GetTask(r.Context(), apptasks.GetTaskInput{OwnerID: p.ID, ID: id})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, taskToResponse(def))
}

func (h *TasksHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	q := r.URL.Query()
	in := apptasks.ListTasksInput{
		OwnerID: p.ID,
		Type:    strings.TrimSpace(q.Get("type")),
		Cursor:  strings.TrimSpace(q.Get("cursor")),
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, r, fmt.Errorf("%w: limit must be a non-negative integer", errInvalidBody))
			return
		}
		in.Limit = n
	}
	if v := strings.TrimSpace(q.Get("enabled")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: enabled must be true or false", errInvalidBody))
			return
		}
		in.Enabled = &b
	}

	page, err := h.svc.ListTasks(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	resp := dto.ListTasksResponse{Items: make([]dto.TaskResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, it := range page.Items {
		resp.Items = append(resp.Items, taskToResponse(it))
	}
	writeJSON(w, http.StatusOK, resp)
}

func validatesteCreateRequest(req dto.CreateTaskRequest) error {
	// Minimal transport-level shape validatestion; business/domain rules live in
	// application/domain (name pattern, enums, ranges, schedule semantics).
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: name is required", errInvalidBody)
	}
	if strings.TrimSpace(req.Type) == "" {
		return fmt.Errorf("%w: type is required", errInvalidBody)
	}
	if strings.TrimSpace(req.Target.Capability) == "" {
		return fmt.Errorf("%w: target.capability is required", errInvalidBody)
	}
	if strings.TrimSpace(req.Schedule.Type) == "" {
		return fmt.Errorf("%w: schedule.type is required", errInvalidBody)
	}
	if len(req.Payload) > 0 && !json.Valid(req.Payload) {
		return fmt.Errorf("%w: payload must be valid JSON", errInvalidBody)
	}
	return nil
}

func bytesOrNil(raw json.RawMessage) []byte {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func taskToResponse(def task.Definition) dto.TaskResponse {
	var payload json.RawMessage
	if len(def.Payload) > 0 && json.Valid(def.Payload) {
		payload = append(json.RawMessage(nil), def.Payload...)
	} else if len(def.Payload) > 0 {
		// Preserve non-JSON bytes as a JSON string to keep response valid JSON.
		b, _ := json.Marshal(string(def.Payload))
		payload = b
	}
	return dto.TaskResponse{
		ID:          def.ID,
		OwnerID:     def.OwnerID,
		Name:        def.Name,
		Type:        def.Type,
		Description: def.Description,
		Payload:     payload,
		Target: dto.TargetResponse{
			Capability: def.Target.Capability,
			WorkerID:   def.Target.WorkerID,
		},
		ExecutionPolicy: dto.ExecutionPolicyResponse{
			TimeoutSeconds: def.ExecutionPolicy.TimeoutSeconds,
			MaxRetries:     def.ExecutionPolicy.MaxRetries,
			Priority:       def.ExecutionPolicy.Priority,
		},
		Schedule: dto.ScheduleResponse{
			Type:          string(def.Schedule.Type),
			Expression:    def.Schedule.Expression,
			Timezone:      def.Schedule.Timezone,
			RunAt:         cloneUTCTime(def.Schedule.RunAt),
			NextRunAt:     cloneUTCTime(def.Schedule.NextRunAt),
			LastRunAt:     cloneUTCTime(def.Schedule.LastRunAt),
			MisfirePolicy: string(def.Schedule.MisfirePolicy),
		},
		Enabled:   def.Enabled,
		DeletedAt: cloneUTCTime(def.DeletedAt),
		CreatedAt: def.CreatedAt.UTC(),
		UpdatedAt: def.UpdatedAt.UTC(),
	}
}

func cloneUTCTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}
