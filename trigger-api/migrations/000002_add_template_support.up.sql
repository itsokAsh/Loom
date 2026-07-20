-- Add template support to workflows table
ALTER TABLE workflows ADD COLUMN fingerprint TEXT UNIQUE;
ALTER TABLE workflows ADD COLUMN template_id TEXT;
ALTER TABLE workflows ADD COLUMN template_version INTEGER;

CREATE INDEX IF NOT EXISTS idx_workflows_fingerprint ON workflows(fingerprint);
CREATE INDEX IF NOT EXISTS idx_workflows_template_id ON workflows(template_id);

-- Add idempotency tracking for webhook triggers
CREATE TABLE IF NOT EXISTS webhook_idempotency (
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    execution_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW() + INTERVAL '24 hours',
    PRIMARY KEY (webhook_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_webhook_idempotency_expires_at ON webhook_idempotency(expires_at);

-- Optional: Add cleanup function for expired idempotency keys
-- This can be called by a cron job or background worker
-- DELETE FROM webhook_idempotency WHERE expires_at < NOW();
