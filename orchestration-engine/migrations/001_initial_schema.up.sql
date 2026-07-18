CREATE TABLE workflow_runs (
    execution_id UUID PRIMARY KEY,
    workflow_id UUID NOT NULL,
    workflow_version INT NOT NULL,
    dag_definition JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'RUNNING',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE node_executions (
    execution_id UUID NOT NULL REFERENCES workflow_runs(execution_id),
    node_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    input_data JSONB,
    output_data JSONB,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (execution_id, node_id)
);

CREATE TABLE dispatched_tasks (
    execution_id UUID NOT NULL,
    node_id TEXT NOT NULL,
    dispatch_id UUID NOT NULL DEFAULT gen_random_uuid(),
    dispatched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempt_timeout_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (execution_id, node_id)
);

CREATE INDEX idx_dispatched_tasks_timeout ON dispatched_tasks (attempt_timeout_at);
