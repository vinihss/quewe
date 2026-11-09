package groups

import (
	"context"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/domain/taskgroup"
	portclock "github.com/vinihss/quewe/services/quewe/internal/ports/clock"
	portid "github.com/vinihss/quewe/services/quewe/internal/ports/id"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type Service struct {
	repo  repositories.GroupRepository
	clock portclock.Clock
	ids   portid.Generator
}

type NewServiceInput struct {
	Repo  repositories.GroupRepository
	Clock portclock.Clock
	IDs   interface{ New() string }
}

func NewService(repo repositories.GroupRepository, clock portclock.Clock, ids portid.Generator) *Service {
	return &Service{repo: repo, clock: clock, ids: ids}
}

type CreateInput struct {
	ParentExecutionID  string
	Policy             taskgroup.Policy
	MinSuccess         int
	Deadline           *time.Time
	MemberExecutionIDs []string
	Required           map[string]bool
}

type CreateOutput struct {
	GroupID string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (CreateOutput, error) {
	if !in.Policy.Valid() {
		return CreateOutput{}, apperr.Validatestion("invalid policy")
	}
	g := taskgroup.Group{
		ID:                s.ids.New(),
		ParentExecutionID: in.ParentExecutionID,
		Policy:            in.Policy,
		MinSuccess:        in.MinSuccess,
		Deadline:          in.Deadline,
		Status:            taskgroup.StatusOpen,
		CreatedAt:         s.clock.Now(),
		UpdatedAt:         s.clock.Now(),
	}
	if err := g.Validateste(); err != nil {
		return CreateOutput{}, apperr.Wrap(apperr.CodeValidatestion, "validateste group", err)
	}

	members := make([]taskgroup.Member, len(in.MemberExecutionIDs))
	for i, eid := range in.MemberExecutionIDs {
		req := true
		if in.Required != nil {
			req = in.Required[eid]
		}
		members[i] = taskgroup.Member{
			GroupID:     g.ID,
			ExecutionID: eid,
			Required:    req,
			JoinedAt:    s.clock.Now(),
		}
	}

	if err := s.repo.Create(ctx, g, members); err != nil {
		return CreateOutput{}, apperr.Wrap(apperr.CodeInternal, "create group", err)
	}
	return CreateOutput{GroupID: g.ID}, nil
}

type GetInput struct {
	GroupID string
}

type GetOutput struct {
	Group   taskgroup.Group
	Status  taskgroup.Status
	Members []taskgroup.Member
}

func (s *Service) Get(ctx context.Context, in GetInput) (GetOutput, error) {
	g, err := s.repo.Get(ctx, in.GroupID)
	if err != nil {
		return GetOutput{}, apperr.Wrap(apperr.CodeNotFound, "group", err)
	}
	members, err := s.repo.GetMembers(ctx, in.GroupID)
	if err != nil {
		return GetOutput{}, apperr.Wrap(apperr.CodeInternal, "get members", err)
	}
	status, err := s.repo.Evaluate(ctx, in.GroupID)
	if err != nil {
		return GetOutput{}, apperr.Wrap(apperr.CodeInternal, "evaluate", err)
	}
	return GetOutput{Group: g, Status: status, Members: members}, nil
}

type EvaluateInput struct {
	GroupID string
}

func (s *Service) Evaluate(ctx context.Context, in EvaluateInput) (taskgroup.Status, error) {
	status, err := s.repo.Evaluate(ctx, in.GroupID)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, "evaluate", err)
	}
	if err := s.repo.UpdateStatus(ctx, in.GroupID, status); err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, "update status", err)
	}
	return status, nil
}
