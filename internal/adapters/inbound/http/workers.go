package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/dto"
	"github.com/vinihss/quewe/services/quewe/internal/application/consuming"
	"github.com/vinihss/quewe/services/quewe/internal/application/workers"
	"github.com/vinihss/quewe/services/quewe/internal/ports/auth"
)

type WorkersHandler struct {
	workers   *workers.Service
	consuming *consuming.Service
	auth      auth.Resolver
}

func NewWorkersHandler(w *workers.Service, c *consuming.Service, resolver auth.Resolver) *WorkersHandler {
	return &WorkersHandler{workers: w, consuming: c, auth: resolver}
}

// Register mounts worker routes on the versioned API mux.
// Contract: contracts/task/openapi.yaml (tag: Workers).
func (h *WorkersHandler) Register(api *http.ServeMux) {
	api.HandleFunc("POST /workers/register", h.register)
	api.HandleFunc("POST /workers/heartbeat", h.heartbeat)
	api.HandleFunc("POST /workers/consume", h.consume)
	api.HandleFunc("GET /workers", h.list)
	api.HandleFunc("GET /workers/{id}", h.get)
	api.HandleFunc("POST /workers/{id}/drain", h.drain)
}

func (h *WorkersHandler) principal(r *http.Request) (auth.Principal, error) {
	ctx := contextWithRequest(r.Context(), r)
	p, err := h.auth.Resolve(ctx)
	if err != nil {
		return auth.Principal{}, err
	}
	return p, nil
}

func (h *WorkersHandler) register(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req dto.RegisterWorkerRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	if strings.TrimSpace(req.WorkerID) == "" {
		req.WorkerID = p.ID
	}
	err = h.workers.Register(r.Context(), workers.RegisterInput{
		WorkerID:     req.WorkerID,
		Hostname:     req.Hostname,
		Version:      req.Version,
		Concurrency:  req.Concurrency,
		Capabilities: req.Capabilities,
		Metadata:     req.Metadata,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"worker_id": req.WorkerID})
}

func (h *WorkersHandler) heartbeat(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req dto.HeartbeatRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := h.workers.Heartbeat(r.Context(), workers.HeartbeatInput{WorkerID: p.ID, Metrics: req.Metrics})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.HeartbeatResponse{CancelRequested: out.CancelRequested})
}

func (h *WorkersHandler) consume(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req dto.ConsumeRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	max := req.Max
	if max <= 0 {
		max = 1
	}
	wait := time.Duration(req.WaitSeconds) * time.Second
	out, err := h.consuming.Consume(r.Context(), consuming.ConsumeInput{
		WorkerID:     p.ID,
		Capabilities: req.Capabilities,
		Max:          max,
		Wait:         wait,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	if len(out.Messages) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	msgs := make([]dto.ConsumedMessage, 0, len(out.Messages))
	for _, m := range out.Messages {
		msgs = append(msgs, dto.ConsumedMessage{
			MessageID:       m.MessageID,
			ExecutionID:     m.ExecutionID,
			TaskID:          m.TaskID,
			Type:            m.Type,
			Capability:      m.Capability,
			Attempt:         m.Attempt,
			MaxAttempts:     m.MaxAttempts,
			LeaseGeneration: m.LeaseGeneration,
			LeaseExpiresAt:  m.LeaseExpiresAt.UTC(),
			TimeoutSeconds:  m.TimeoutSeconds,
			IdempotencyKey:  m.IdempotencyKey,
			Payload:         json.RawMessage(m.Payload),
			TraceContext:    m.TraceContext,
		})
	}
	writeJSON(w, http.StatusOK, dto.ConsumeResponse{Messages: msgs})
}

func (h *WorkersHandler) list(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	ws, err := h.workers.List(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]dto.WorkerResponse, 0, len(ws))
	for _, wk := range ws {
		out = append(out, dto.WorkerResponse{
			WorkerID:        wk.WorkerID,
			Hostname:        wk.Hostname,
			Version:         wk.Version,
			Status:          string(wk.Status),
			Concurrency:     wk.Concurrency,
			LastHeartbeatAt: wk.LastHeartbeatAt,
			Capabilities:    wk.Capabilities,
			Metadata:        wk.Metadata,
		})
	}
	writeJSON(w, http.StatusOK, dto.WorkersListResponse{Workers: out})
}

func (h *WorkersHandler) get(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	wk, err := h.workers.Get(r.Context(), workers.GetInput{WorkerID: id})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.WorkerResponse{
		WorkerID:        wk.WorkerID,
		Hostname:        wk.Hostname,
		Version:         wk.Version,
		Status:          string(wk.Status),
		Concurrency:     wk.Concurrency,
		LastHeartbeatAt: wk.LastHeartbeatAt,
		Capabilities:    wk.Capabilities,
		Metadata:        wk.Metadata,
	})
}

func (h *WorkersHandler) drain(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if err := h.workers.Drain(r.Context(), workers.DrainInput{WorkerID: id}); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
