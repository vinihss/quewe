CREATE TABLE IF NOT EXISTS queue_messages (
    id UUID PRIMARY KEY,
    execution_id UUID NOT NULL,
    queue TEXT NOT NULL,
    capability TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 9),
    available_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','reserved','completed','dead_letter')),
    attempts INT NOT NULL DEFAULT 0,
    reserved_by UUID,
    lease_expires_at TIMESTAMPTZ,
    lease_generation INT NOT NULL DEFAULT 0,
    trace_context JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (execution_id)
);

CREATE INDEX IF NOT EXISTS idx_queue_ready ON queue_messages (status, available_at);
CREATE INDEX IF NOT EXISTS idx_queue_reserved ON queue_messages (status, lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_queue_capability ON queue_messages (capability, status, available_at);

CREATE TABLE IF NOT EXISTS workers (
    worker_id TEXT PRIMARY KEY,
    hostname TEXT,
    version TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','draining','offline')),
    concurrency INT NOT NULL DEFAULT 1,
    last_heartbeat_at TIMESTAMPTZ,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB
);

CREATE TABLE IF NOT EXISTS worker_capabilities (
    worker_id TEXT NOT NULL REFERENCES workers(worker_id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    PRIMARY KEY (worker_id, capability)
);
