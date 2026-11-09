CREATE TABLE IF NOT EXISTS principals (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    token_hash TEXT,
    scopes TEXT[],
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES principals(id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT,
    payload JSONB,
    target_capability TEXT NOT NULL,
    target_worker_id UUID,
    timeout_seconds INT NOT NULL,
    max_retries INT NOT NULL,
    priority INT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, name)
);

CREATE INDEX IF NOT EXISTS idx_tasks_owner_created ON tasks(owner_id, created_at DESC);

CREATE TABLE IF NOT EXISTS task_schedules (
    task_id UUID PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    expression TEXT,
    timezone TEXT,
    run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    misfire_policy TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS executions (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id),
    owner_id UUID NOT NULL REFERENCES principals(id),
    scheduled_for TIMESTAMPTZ,
    status TEXT NOT NULL,
    attempt INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_id, scheduled_for)
);

CREATE INDEX IF NOT EXISTS idx_executions_task_created ON executions(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_executions_owner_created ON executions(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_executions_status ON executions(status);
