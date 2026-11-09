package http

import (
	"net/http"
	"strings"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/dto"
	"github.com/vinihss/quewe/services/quewe/internal/application/groups"
	"github.com/vinihss/quewe/services/quewe/internal/domain/taskgroup"
	"github.com/vinihss/quewe/services/quewe/internal/ports/auth"
)

type GroupsHandler struct {
	groups *groups.Service
	auth   auth.Resolver
}

func NewGroupsHandler(g *groups.Service, resolver auth.Resolver) *GroupsHandler {
	return &GroupsHandler{groups: g, auth: resolver}
}

// Register mounts group routes on the versioned API mux.
// Contract: contracts/task/openapi.yaml (tag: Groups).
func (h *GroupsHandler) Register(api *http.ServeMux) {
	api.HandleFunc("POST /groups", h.create)
	api.HandleFunc("GET /groups/{id}", h.get)
	api.HandleFunc("POST /groups/{id}/evaluate", h.evaluate)
}

func (h *GroupsHandler) principal(r *http.Request) (auth.Principal, error) {
	ctx := contextWithRequest(r.Context(), r)
	p, err := h.auth.Resolve(ctx)
	if err != nil {
		return auth.Principal{}, err
	}
	return p, nil
}

func (h *GroupsHandler) create(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req dto.CreateGroupRequest
	if err := decodeJSON(w, r, &req, maxJSONBodyBytes); err != nil {
		writeError(w, r, err)
		return
	}
	out, err := h.groups.Create(r.Context(), groups.CreateInput{
		ParentExecutionID:  req.ParentExecutionID,
		Policy:             taskgroup.Policy(req.Policy),
		MinSuccess:         req.MinSuccess,
		Deadline:           req.Deadline,
		MemberExecutionIDs: req.MemberExecutionIDs,
		Required:           req.Required,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"group_id": out.GroupID})
}

func (h *GroupsHandler) get(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	out, err := h.groups.Get(r.Context(), groups.GetInput{GroupID: id})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.GroupResponse{
		GroupID: out.Group.ID,
		Status:  string(out.Status),
		Members: out.Members,
	})
}

func (h *GroupsHandler) evaluate(w http.ResponseWriter, r *http.Request) {
	_, err := h.principal(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	status, err := h.groups.Evaluate(r.Context(), groups.EvaluateInput{GroupID: id})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(status)})
}
