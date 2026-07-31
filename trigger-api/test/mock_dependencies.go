package test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	contracts "github.com/loom/shared/queue-contracts"
	"github.com/loom/trigger-api/internal/db"
)

type MockWebhookStore struct {
	GetWebhookByPathFn                      func(ctx context.Context, path string) (db.Webhook, error)
	GetLatestWorkflowVersionFn              func(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error)
	GetWorkflowByIDFn                       func(ctx context.Context, workflowID pgtype.UUID) (db.Workflow, error)
	CreateExecutionFn                       func(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error)
	GetExecutionByWorkflowAndIdempotencyKeyFn func(ctx context.Context, arg db.GetExecutionByWorkflowAndIdempotencyKeyParams) (db.Execution, error)
	GetIdempotentExecutionFn                func(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string) (pgtype.UUID, bool, error)
	SaveIdempotentExecutionFn               func(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string, executionID pgtype.UUID, expiresAt time.Time) (bool, error)
}

func (m *MockWebhookStore) GetWebhookByPath(ctx context.Context, path string) (db.Webhook, error) {
	if m.GetWebhookByPathFn != nil {
		return m.GetWebhookByPathFn(ctx, path)
	}
	return db.Webhook{}, nil
}

func (m *MockWebhookStore) GetLatestWorkflowVersion(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error) {
	if m.GetLatestWorkflowVersionFn != nil {
		return m.GetLatestWorkflowVersionFn(ctx, workflowID)
	}
	return db.WorkflowVersion{}, nil
}

func (m *MockWebhookStore) GetWorkflowByID(ctx context.Context, workflowID pgtype.UUID) (db.Workflow, error) {
	if m.GetWorkflowByIDFn != nil {
		return m.GetWorkflowByIDFn(ctx, workflowID)
	}
	return db.Workflow{}, nil
}

func (m *MockWebhookStore) GetIdempotentExecution(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string) (pgtype.UUID, bool, error) {
	if m.GetIdempotentExecutionFn != nil {
		return m.GetIdempotentExecutionFn(ctx, webhookID, idempotencyKey)
	}
	return pgtype.UUID{}, false, nil
}

func (m *MockWebhookStore) SaveIdempotentExecution(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string, executionID pgtype.UUID, expiresAt time.Time) (bool, error) {
	if m.SaveIdempotentExecutionFn != nil {
		return m.SaveIdempotentExecutionFn(ctx, webhookID, idempotencyKey, executionID, expiresAt)
	}
	return true, nil
}

func (m *MockWebhookStore) CreateExecution(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error) {
	if m.CreateExecutionFn != nil {
		return m.CreateExecutionFn(ctx, arg)
	}
	return db.Execution{}, nil
}

func (m *MockWebhookStore) GetExecutionByWorkflowAndIdempotencyKey(ctx context.Context, arg db.GetExecutionByWorkflowAndIdempotencyKeyParams) (db.Execution, error) {
	if m.GetExecutionByWorkflowAndIdempotencyKeyFn != nil {
		return m.GetExecutionByWorkflowAndIdempotencyKeyFn(ctx, arg)
	}
	return db.Execution{}, nil
}

type MockRunPublisher struct {
	PublishNewRunFn func(ctx context.Context, msg contracts.NewRunMessage) error
	PublishedCount  int
}

func (m *MockRunPublisher) PublishNewRun(ctx context.Context, msg contracts.NewRunMessage) error {
	m.PublishedCount++
	if m.PublishNewRunFn != nil {
		return m.PublishNewRunFn(ctx, msg)
	}
	return nil
}
