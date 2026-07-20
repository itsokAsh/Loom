-- Reverse template support migrations

DROP TABLE IF EXISTS webhook_idempotency;

DROP INDEX IF EXISTS idx_workflows_template_id;
DROP INDEX IF EXISTS idx_workflows_fingerprint;

ALTER TABLE workflows DROP COLUMN IF EXISTS template_version;
ALTER TABLE workflows DROP COLUMN IF EXISTS template_id;
ALTER TABLE workflows DROP COLUMN IF EXISTS fingerprint;
