CREATE TABLE IF NOT EXISTS task_groups (
    id UUID PRIMARY KEY,
    parent_execution_id UUID NOT NULL,
    policy TEXT NOT NULL CHECK (policy IN ('all_completed','all_settled','minimum_success','first_success')),
    min_success INT NOT NULL DEFAULT 0,
    deadline TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','completed','completed_partial','failed','cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_task_groups_parent ON task_groups (parent_execution_id);
CREATE INDEX IF NOT EXISTS idx_task_groups_status ON task_groups (status);

CREATE TABLE IF NOT EXISTS group_members (
    group_id UUID NOT NULL REFERENCES task_groups(id) ON DELETE CASCADE,
    execution_id UUID NOT NULL REFERENCES executions(id),
    required BOOLEAN NOT NULL DEFAULT TRUE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, execution_id)
);

CREATE INDEX IF NOT EXISTS idx_group_members_execution ON group_members (execution_id);