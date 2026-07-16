package workflows

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
)

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateWorkflow(ctx context.Context, name string, dag []byte) (*db.Workflow, *db.WorkflowVersion, error) {
	tx, err := s.store.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.store.WithTx(tx)

	wf, err := qtx.CreateWorkflow(ctx, name)
	if err != nil {
		return nil, nil, err
	}

	wv, err := qtx.CreateWorkflowVersion(ctx, db.CreateWorkflowVersionParams{
		WorkflowID:    wf.ID,
		Version:       1,
		DagDefinition: dag,
	})
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return &wf, &wv, nil
}

func (s *Service) GetWorkflow(ctx context.Context, id pgtype.UUID) (*db.Workflow, error) {
	wf, err := s.store.GetWorkflow(ctx, id)
	return &wf, err
}

func (s *Service) AddVersion(ctx context.Context, workflowID pgtype.UUID, version int32, dag []byte) (*db.WorkflowVersion, error) {
	wv, err := s.store.CreateWorkflowVersion(ctx, db.CreateWorkflowVersionParams{
		WorkflowID:    workflowID,
		Version:       version,
		DagDefinition: dag,
	})
	return &wv, err
}

func (s *Service) GetStore() *db.Store {
	return s.store
}
