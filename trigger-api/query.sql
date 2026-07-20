-- name: CreateWorkflow :one
INSERT INTO workflows (name)
VALUES ($1)
RETURNING *;

-- name: CreateWorkflowVersion :one
INSERT INTO workflow_versions (workflow_id, version, dag_definition)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkflow :one
SELECT * FROM workflows
WHERE id = $1 LIMIT 1;

-- name: GetWorkflowVersion :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1 AND version = $2 LIMIT 1;

-- name: GetLatestWorkflowVersion :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1
ORDER BY version DESC LIMIT 1;

-- name: CreateWebhook :one
INSERT INTO webhooks (workflow_id, path, secret)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWebhookByPath :one
SELECT * FROM webhooks
WHERE path = $1 LIMIT 1;

-- name: CreateSchedule :one
INSERT INTO schedules (workflow_id, cron_expression, next_run_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ClaimDueSchedules :many
UPDATE schedules
SET leased_by = $1, lease_expires_at = $2
WHERE id IN (
    SELECT s.id FROM schedules s
    WHERE s.next_run_at <= $3 AND (s.leased_by IS NULL OR s.lease_expires_at < $3)
    FOR UPDATE SKIP LOCKED
    LIMIT $4
)
RETURNING *;

-- name: UpdateScheduleNextRun :exec
UPDATE schedules
SET next_run_at = $2, leased_by = NULL, lease_expires_at = NULL
WHERE id = $1;

-- name: CreateExecution :one
INSERT INTO executions (workflow_id, workflow_version, idempotency_key, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (workflow_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetExecutionByWorkflowAndIdempotencyKey :one
SELECT * FROM executions
WHERE workflow_id = $1 AND idempotency_key = $2 LIMIT 1;

-- name: GetExecution :one
SELECT * FROM executions
WHERE id = $1 LIMIT 1;

-- name: ListExecutions :many
SELECT * FROM executions
WHERE workflow_id = $1
AND (sqlc.narg('cursor')::timestamptz IS NULL OR created_at < sqlc.narg('cursor'))
ORDER BY created_at DESC
LIMIT $2;

-- name: UpdateExecutionStatus :exec
UPDATE executions
SET status = $2,
    updated_at = $3,
    completed_at = COALESCE($4, completed_at)
WHERE id = $1;

-- Template-related queries

-- name: CreateWorkflowFromTemplate :one
INSERT INTO workflows (name, fingerprint, template_id, template_version)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: FindWorkflowByFingerprint :one
SELECT * FROM workflows
WHERE fingerprint = $1 LIMIT 1;

-- name: GetWebhookByWorkflowID :one
SELECT * FROM webhooks
WHERE workflow_id = $1 LIMIT 1;

-- name: SaveIdempotentExecution :one
INSERT INTO webhook_idempotency (webhook_id, idempotency_key, execution_id, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (webhook_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetIdempotentExecution :one
SELECT execution_id FROM webhook_idempotency
WHERE webhook_id = $1 AND idempotency_key = $2 AND expires_at > NOW()
LIMIT 1;

-- name: CleanupExpiredIdempotencyKeys :exec
DELETE FROM webhook_idempotency
WHERE expires_at < NOW();
