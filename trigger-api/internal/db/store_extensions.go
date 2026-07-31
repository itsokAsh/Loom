package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetWorkflowByID wraps GetWorkflow to match the WebhookStore interface.
func (s *Store) GetWorkflowByID(ctx context.Context, workflowID pgtype.UUID) (Workflow, error) {
	return s.GetWorkflow(ctx, workflowID)
}

// GetIdempotentExecution matches the handler signature.
func (s *Store) GetIdempotentExecution(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string) (pgtype.UUID, bool, error) {
	execID, err := s.Queries.GetIdempotentExecution(ctx, GetIdempotentExecutionParams{
		WebhookID:      webhookID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, false, nil
		}
		return pgtype.UUID{}, false, err
	}
	return execID, true, nil
}

// SaveIdempotentExecution executes the atomic insert. Returns true if inserted, false if conflict.
func (s *Store) SaveIdempotentExecution(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string, executionID pgtype.UUID, expiresAt time.Time) (bool, error) {
	_, err := s.Queries.SaveIdempotentExecution(ctx, SaveIdempotentExecutionParams{
		WebhookID:      webhookID,
		IdempotencyKey: idempotencyKey,
		ExecutionID:    executionID,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListWorkflows returns all workflows ordered by created_at desc.
func (s *Store) ListWorkflows(ctx context.Context) ([]Workflow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, created_at, fingerprint, template_id, template_version
		FROM workflows ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workflow
	for rows.Next() {
		var w Workflow
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedAt, &w.Fingerprint, &w.TemplateID, &w.TemplateVersion); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// UpdateWorkflowName sets the workflow display name.
func (s *Store) UpdateWorkflowName(ctx context.Context, id pgtype.UUID, name string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE workflows SET name = $2 WHERE id = $1`, id, name)
	return err
}

// DeleteWorkflow removes a workflow and cascaded children.
func (s *Store) DeleteWorkflow(ctx context.Context, id pgtype.UUID) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	return err
}

// ListWebhooksByWorkflow returns all webhooks for a workflow.
func (s *Store) ListWebhooksByWorkflow(ctx context.Context, workflowID pgtype.UUID) ([]Webhook, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, workflow_id, path, secret, created_at FROM webhooks
		WHERE workflow_id = $1 ORDER BY created_at`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		if err := rows.Scan(&w.ID, &w.WorkflowID, &w.Path, &w.Secret, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ListSchedulesByWorkflow returns cron schedules for a workflow.
func (s *Store) ListSchedulesByWorkflow(ctx context.Context, workflowID pgtype.UUID) ([]Schedule, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, workflow_id, cron_expression, next_run_at, leased_by, lease_expires_at, created_at
		FROM schedules WHERE workflow_id = $1 ORDER BY created_at`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		var sch Schedule
		if err := rows.Scan(&sch.ID, &sch.WorkflowID, &sch.CronExpression, &sch.NextRunAt,
			&sch.LeasedBy, &sch.LeaseExpiresAt, &sch.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sch)
	}
	return out, rows.Err()
}

// DeleteSchedulesByWorkflow removes all cron schedules for a workflow.
func (s *Store) DeleteSchedulesByWorkflow(ctx context.Context, workflowID pgtype.UUID) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM schedules WHERE workflow_id = $1`, workflowID)
	return err
}

// DeleteSchedule removes one schedule row for a workflow.
func (s *Store) DeleteSchedule(ctx context.Context, workflowID, scheduleID pgtype.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM schedules WHERE id = $1 AND workflow_id = $2`, scheduleID, workflowID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
