package templates

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
)

type storeAdapter struct {
	store *db.Store
}

func NewStoreAdapter(store *db.Store) Store {
	return &storeAdapter{store: store}
}

func (a *storeAdapter) FindWorkflowByFingerprint(ctx context.Context, fingerprint string) (*WorkflowWithWebhook, bool, error) {
	wf, err := a.store.Queries.FindWorkflowByFingerprint(ctx, pgtype.Text{String: fingerprint, Valid: true})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &WorkflowWithWebhook{ID: fmt.Sprintf("%x", wf.ID.Bytes)}, true, nil
}

func (a *storeAdapter) CreateWorkflowFromTemplate(ctx context.Context, params CreateWorkflowFromTemplateParams) (string, error) {
	var wf db.Workflow
	var err error

	// start tx
	tx, err := a.store.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	qtx := a.store.WithTx(tx)

	wf, err = qtx.CreateWorkflowFromTemplate(ctx, db.CreateWorkflowFromTemplateParams{
		Name:            params.Name,
		Fingerprint:     pgtype.Text{String: params.Fingerprint, Valid: true},
		TemplateID:      pgtype.Text{String: params.TemplateID, Valid: true},
		TemplateVersion: pgtype.Int4{Int32: int32(params.TemplateVersion), Valid: true},
	})
	if err != nil {
		return "", err
	}

	_, err = qtx.CreateWorkflowVersion(ctx, db.CreateWorkflowVersionParams{
		WorkflowID:    wf.ID,
		Version:       1,
		DagDefinition: params.DAG,
	})
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", wf.ID.Bytes), nil
}

func (a *storeAdapter) CreateWebhook(ctx context.Context, params CreateWebhookParams) (*WebhookResponse, error) {
	var workflowID pgtype.UUID
	workflowID.Scan(params.WorkflowID)

	wh, err := a.store.Queries.CreateWebhook(ctx, db.CreateWebhookParams{
		WorkflowID: workflowID,
		Path:       params.Path,
		Secret:     params.Secret,
	})
	if err != nil {
		return nil, err
	}

	return &WebhookResponse{
		ID:     fmt.Sprintf("%x", wh.ID.Bytes),
		Path:   wh.Path,
		Secret: wh.Secret,
	}, nil
}

func (a *storeAdapter) GetWebhookByWorkflowID(ctx context.Context, workflowID string) (*WebhookResponse, error) {
	var wfID pgtype.UUID
	wfID.Scan(workflowID)

	wh, err := a.store.Queries.GetWebhookByWorkflowID(ctx, wfID)
	if err != nil {
		return nil, err
	}
	return &WebhookResponse{
		ID:     fmt.Sprintf("%x", wh.ID.Bytes),
		Path:   wh.Path,
		Secret: wh.Secret,
	}, nil
}
