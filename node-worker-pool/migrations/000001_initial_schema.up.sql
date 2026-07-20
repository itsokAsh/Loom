CREATE TABLE IF NOT EXISTS external_call_log (
    execution_id VARCHAR(255) NOT NULL,
    node_id VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL, -- 'IN_PROGRESS', 'COMPLETED'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (execution_id, node_id)
);

CREATE INDEX idx_external_call_log_idempotency_key ON external_call_log(idempotency_key);
