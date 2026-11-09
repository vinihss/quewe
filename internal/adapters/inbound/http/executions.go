package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/dto"
	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/ports/auth"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type ExecutionRepository interface {
	GetByID(ctx context.Context, ownerID, executionID string) (execution.Execution, error)
	List(ctx context.Context, filter repositories.ExecutionListFilter) (repositories.ExecutionPage, error)
}

type ExecutionsHandler struct {
	repo ExecutionRepository
	auth auth.Resolver
}

func NewExecutionsHandler(repo ExecutionRepository, resolver auth.Resolver) *ExecutionsHandler {
	return &ExecutionsHandler{repo: repo, auth: resolver}
}

func (h *ExecutionsHandler) Register(api *http.ServeMux) {
	api.HandleFunc("GET /executions", h.listExecutions)
	api.HandleFunc("GET /executions/{id}", h.getExecution)
}

func (h *ExecutionsHandler) principal(r *http.Request) (auth.Principal, error) {
	ctx := contextWithRequest(r.Context(), r)
	p, err := h.auth.Resolve(ctx)
	if err != nil {
		return auth.Principal{}, err
	}
	return p, nil
}

func (h *ExecutionsHandler) getExecution(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, r, fmt.Errorf("%w: execution id is required", errInvalidBody))
		return
	}
	ex, err := h.repo.GetByID(r.Context(), p.ID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, executionToResponse(ex))
}

func (h *ExecutionsHandler) listExecutions(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	q := r.URL.Query()
	filter := repositories.ExecutionListFilter{OwnerID: p.ID, Cursor: strings.TrimSpace(q.Get("cursor"))}
	if v := strings.TrimSpace(q.Get("task_id")); v != "" {
		filter.TaskID = &v
	}
	if v := strings.TrimSpace(q.Get("status")); v != "" {
		st := execution.Status(v)
		if !st.Valid() {
			writeError(w, r, fmt.Errorf("%w: invalid status", errInvalidBody))
			return
		}
		filter.Status = &st
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, r, fmt.Errorf("%w: limit must be a non-negative integer", errInvalidBody))
			return
		}
		filter.Limit = n
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: invalid from", errInvalidBody))
			return
		}
		filter.From = &t
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: invalid to", errInvalidBody))
			return
		}
		filter.To = &t
	}
	page, err := h.repo.List(r.Context(), filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	resp := dto.ListExecutionsResponse{Items: make([]dto.ExecutionResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, it := range page.Items {
		resp.Items = append(resp.Items, executionToResponse(it))
	}
	writeJSON(w, http.StatusOK, resp)
}

func executionToResponse(e execution.Execution) dto.ExecutionResponse {
	return dto.ExecutionResponse{
		ID:                e.ID,
		TaskID:            e.TaskID,
		Status:            string(e.Status),
		Attempt:           e.Attempt,
		MaxAttempts:       e.MaxAttempts,
		ScheduledFor:      e.ScheduledFor.UTC(),
		WorkerID:          e.WorkerID,
		StartedAt:         cloneUTCTimePtr(e.StartedAt),
		FinishedAt:        cloneUTCTimePtr(e.FinishedAt),
		CancelRequestedAt: cloneUTCTimePtr(e.CancelRequestedAt),
		RedriveCount:      e.RedriveCount,
		ResultRef:         e.ResultRef,
		ErrorCode:         e.ErrorCode,
		ErrorMessage:      e.ErrorMessage,
		CreatedAt:         e.CreatedAt.UTC(),
		UpdatedAt:         e.UpdatedAt.UTC(),
	}
}

func cloneUTCTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}
