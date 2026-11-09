package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/dto"
	"github.com/vinihss/quewe/services/quewe/internal/application/consuming"
	"github.com/vinihss/quewe/services/quewe/internal/application/leasing"
	"github.com/vinihss/quewe/services/quewe/internal/ports/auth"
)

type ExecutionsBrokerHandler struct {
	consuming *consuming.Service
	leasing   *leasing.Service
	auth      auth.Resolver
}

func NewExecutionsBrokerHandler(c *consuming.Service, l *leasing.Service, resolver auth.Resolver) *ExecutionsBrokerHandler {
	return &ExecutionsBrokerHandler{consuming: c, leasing: l, auth: resolver}
}

func (h *ExecutionsBrokerHandler) Register(api *http.ServeMux) {
	api.HandleFunc("POST /executions/{id}/heartbeat", h.renew)
	api.HandleFunc("POST /executions/{id}/result", h.finish)
	api.HandleFunc("POST /executions/{id}/ack", h.ack)
	api.HandleFunc("POST /executions/{id}/nack", h.nack)
	api.HandleFunc("POST /executions/{id}/cancel", h.cancel)
}

type FinishInput struct {
	LeaseGeneration int
	Success         bool
	InlineBody      *string
	ArtifactURI     *string
}

func (h *ExecutionsBrokerHandler) finish(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req FinishInput
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	in := consuming.FinishInput{ExecutionID: id, LeaseGeneration: req.LeaseGeneration, Result: consuming.FinishResult{Success: req.Success, InlineBody: req.InlineBody, ArtifactURI: req.ArtifactURI}}
	if err := h.consuming.Finish(r.Context(), in); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExecutionsBrokerHandler) renew(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req dto.RenewLeaseRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	exp, err := time.Parse(time.RFC3339, req.LeaseExpiresAt)
	if err != nil {
		writeError(w, r, fmt.Errorf("%w: invalid lease_expires_at", errInvalidBody))
		return
	}
	err = h.leasing.Renew(r.Context(), leasing.RenewInput{ExecutionID: id, LeaseGeneration: req.LeaseGeneration, NewExpiry: exp})
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExecutionsBrokerHandler) ack(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req dto.AckRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	err = h.consuming.Ack(r.Context(), consuming.AckInput{ExecutionID: id, LeaseGeneration: req.LeaseGeneration})
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExecutionsBrokerHandler) nack(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req dto.NackRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	err = h.consuming.Nack(r.Context(), consuming.NackInput{ExecutionID: id, LeaseGeneration: req.LeaseGeneration, Error: consuming.ExecutionError{Code: req.ErrorCode, Message: req.ErrorMessage}, Retryable: req.Retryable})
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExecutionsBrokerHandler) cancel(w http.ResponseWriter, r *http.Request) {
	_, err := h.auth.Resolve(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	in := consuming.CancelInput{ExecutionID: id}
	if err := h.consuming.Cancel(r.Context(), in); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExecutionsBrokerHandler) principal(r *http.Request) (auth.Principal, error) {
	p, err := h.auth.Resolve(r.Context())
	if err != nil {
		return auth.Principal{}, err
	}
	return p, nil
}
