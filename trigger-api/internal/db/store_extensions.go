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
			return false, nil // Conflict, not inserted
		}
		return false, err
	}
	return true, nil // Inserted
}
