-- workflows: versioned, immutable per version
CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workflow_versions (
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    version INT NOT NULL,
    dag_definition JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workflow_id, version)
);

-- webhooks: unguessable path, secret for future HMAC verification
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    path TEXT NOT NULL UNIQUE,       -- random base62 token, not sequential
    secret TEXT NOT NULL,            -- for later inbound signature verification
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- schedules: replaces Redis-locked cron entirely
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    cron_expression TEXT NOT NULL,
    next_run_at TIMESTAMPTZ NOT NULL,
    leased_by TEXT,
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_schedules_due ON schedules (next_run_at) WHERE leased_by IS NULL;

-- executions: idempotent, version-pinned, queryable
CREATE TABLE executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    workflow_version INT NOT NULL,
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workflow_id, idempotency_key)
);
CREATE INDEX idx_executions_status_created ON executions (status, created_at);
