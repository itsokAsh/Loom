-- name: InsertWorkflowRun :exec
INSERT INTO workflow_runs (
    execution_id, workflow_id, workflow_version, dag_definition, status, started_at
) VALUES (
    $1, $2, $3, $4, $5, $6
) ON CONFLICT (execution_id) DO NOTHING;

-- name: GetWorkflowRun :one
SELECT * FROM workflow_runs
WHERE execution_id = $1 FOR UPDATE;

-- name: UpdateWorkflowRunStatus :exec
UPDATE workflow_runs
SET status = $2, updated_at = now(), completed_at = $3
WHERE execution_id = $1;

-- name: InsertNodeExecution :exec
INSERT INTO node_executions (
    execution_id, node_id, status, max_attempts
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (execution_id, node_id) DO NOTHING;

-- name: GetNodeExecution :one
SELECT * FROM node_executions
WHERE execution_id = $1 AND node_id = $2;

-- name: UpdateNodeExecutionStatus :exec
UPDATE node_executions
SET status = $3, output_data = $4, error_message = $5, updated_at = now(), completed_at = $6
WHERE execution_id = $1 AND node_id = $2 AND status IN ('QUEUED', 'RUNNING');

-- name: ListAllNodeExecutions :many
SELECT * FROM node_executions
WHERE execution_id = $1;

-- name: ListCompletedNodeExecutions :many
SELECT * FROM node_executions
WHERE execution_id = $1 AND status IN ('SUCCESS', 'SKIPPED');

-- name: InsertDispatchedTask :one
INSERT INTO dispatched_tasks (
    execution_id, node_id, attempt_timeout_at
) VALUES (
    $1, $2, $3
) RETURNING dispatch_id;

-- name: GetDispatchedTask :one
SELECT * FROM dispatched_tasks
WHERE execution_id = $1 AND node_id = $2;

-- name: DeleteDispatchedTask :exec
DELETE FROM dispatched_tasks
WHERE execution_id = $1 AND node_id = $2;

-- name: InsertOutboxMessage :exec
INSERT INTO outbox_messages (queue, payload)
VALUES ($1, $2);

-- name: ClaimUnpublishedMessages :many
SELECT id, queue, payload, created_at, published_at FROM outbox_messages
WHERE published_at IS NULL
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxMessagePublished :exec
UPDATE outbox_messages
SET published_at = now()
WHERE id = $1;
