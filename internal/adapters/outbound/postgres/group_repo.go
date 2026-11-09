package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinihss/quewe/services/quewe/internal/domain/execution"
	"github.com/vinihss/quewe/services/quewe/internal/domain/taskgroup"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type GroupRepository struct{ pool *pgxpool.Pool }

func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{pool: pool}
}

func (r *GroupRepository) Create(ctx context.Context, g taskgroup.Group, members []taskgroup.Member) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	deadline := g.Deadline
	_, err = tx.Exec(ctx,
		`INSERT INTO task_groups (id, parent_execution_id, policy, min_success, deadline, status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		g.ID, g.ParentExecutionID, string(g.Policy), g.MinSuccess, deadline, string(g.Status), g.CreatedAt, g.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}

	for _, m := range members {
		_, err = tx.Exec(ctx,
			`INSERT INTO group_members (group_id, execution_id, required, joined_at)
			 VALUES ($1,$2,$3,$4)`,
			m.GroupID, m.ExecutionID, m.Required, m.JoinedAt)
		if err != nil {
			return fmt.Errorf("create member: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *GroupRepository) Get(ctx context.Context, groupID string) (taskgroup.Group, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, parent_execution_id, policy, min_success, deadline, status, created_at, updated_at FROM task_groups WHERE id = $1`, groupID)
	return r.scanGroup(row)
}

func (r *GroupRepository) ListByParent(ctx context.Context, parentExecutionID string) ([]taskgroup.Group, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, parent_execution_id, policy, min_success, deadline, status, created_at, updated_at FROM task_groups WHERE parent_execution_id = $1`, parentExecutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []taskgroup.Group
	for rows.Next() {
		g, err := r.scanGroupRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *GroupRepository) UpdateStatus(ctx context.Context, groupID string, status taskgroup.Status) error {
	ct, err := r.pool.Exec(ctx, `UPDATE task_groups SET status=$1, updated_at=now() WHERE id=$2`, string(status), groupID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repositories.ErrNotFound
	}
	return nil
}

func (r *GroupRepository) AddMember(ctx context.Context, groupID, executionID string, required bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO group_members (group_id, execution_id, required, joined_at) VALUES ($1,$2,$3,now())`,
		groupID, executionID, required)
	return err
}

func (r *GroupRepository) GetMembers(ctx context.Context, groupID string) ([]taskgroup.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT group_id, execution_id, required, joined_at FROM group_members WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []taskgroup.Member
	for rows.Next() {
		var m taskgroup.Member
		if err := rows.Scan(&m.GroupID, &m.ExecutionID, &m.Required, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *GroupRepository) Evaluate(ctx context.Context, groupID string) (taskgroup.Status, error) {
	g, err := r.Get(ctx, groupID)
	if err != nil {
		return "", err
	}
	members, err := r.GetMembers(ctx, groupID)
	if err != nil {
		return "", err
	}
	if len(members) == 0 {
		return g.Status, nil
	}

	statuses := make([]taskgroup.MemberStatus, len(members))
	required := make([]bool, len(members))
	for i, m := range members {
		var exStatus string
		err := r.pool.QueryRow(ctx, `SELECT status FROM executions WHERE id = $1`, m.ExecutionID).Scan(&exStatus)
		if err != nil {
			statuses[i] = taskgroup.MemberStatusPending
		} else {
			statuses[i] = mapExecStatusToMemberStatus(exStatus)
		}
		required[i] = m.Required
	}

	return taskgroup.Evaluate(g.Policy, statuses, required, g.MinSuccess), nil
}

func mapExecStatusToMemberStatus(s string) taskgroup.MemberStatus {
	switch s {
	case "completed":
		return taskgroup.MemberStatusSuccess
	case "failed", "dead_letter":
		return taskgroup.MemberStatusFailed
	case "cancelled":
		return taskgroup.MemberStatusCancelled
	case "running", "retrying":
		return taskgroup.MemberStatusRunning
	default:
		return taskgroup.MemberStatusPending
	}
}

func (r *GroupRepository) scanGroup(row interface{ Scan(...any) error }) (taskgroup.Group, error) {
	var g taskgroup.Group
	var policy, status string
	var deadline *time.Time
	var minSuccess int
	err := row.Scan(&g.ID, &g.ParentExecutionID, &policy, &minSuccess, &deadline, &status, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return taskgroup.Group{}, err
	}
	g.Policy = taskgroup.Policy(policy)
	g.MinSuccess = minSuccess
	g.Status = taskgroup.Status(status)
	g.Deadline = deadline
	return g, nil
}

func (r *GroupRepository) scanGroupRows(rows interface{ Scan(...any) error }) (taskgroup.Group, error) {
	return r.scanGroup(rows)
}

var _ = json.Marshal
var _ = repositories.ErrNotFound
var _ = execution.Execution{}
