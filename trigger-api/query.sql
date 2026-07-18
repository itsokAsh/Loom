-- name: CreateWorkflow :one
INSERT INTO workflows (name)
VALUES ($1)
RETURNING id, name, created_at;

-- name: CreateWorkflowVersion :one
INSERT INTO workflow_versions (workflow_id, version, dag_definition)
VALUES ($1, $2, $3)
RETURNING workflow_id, version, dag_definition, created_at;

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
RETURNING id, workflow_id, path, secret, created_at;

-- name: GetWebhookByPath :one
SELECT * FROM webhooks
WHERE path = $1 LIMIT 1;

-- name: CreateSchedule :one
INSERT INTO schedules (workflow_id, cron_expression, next_run_at)
VALUES ($1, $2, $3)
RETURNING id, workflow_id, cron_expression, next_run_at, created_at;

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
RETURNING id, workflow_id, workflow_version, idempotency_key, status, started_at, completed_at, created_at, updated_at;

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
