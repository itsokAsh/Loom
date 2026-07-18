package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	*Queries
	db *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	return &Store{
		Queries: New(pool),
		db:      pool,
	}, nil
}

func (s *Store) Close() {
	s.db.Close()
}

// WithRunLock executes a function inside a transaction where the workflow_run row is locked FOR UPDATE.
func (s *Store) WithRunLock(ctx context.Context, executionID pgtype.UUID, fn func(context.Context, *Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)

	// Lock the run row
	_, err = qtx.GetWorkflowRun(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to lock workflow run: %w", err)
	}

	if err := fn(ctx, qtx); err != nil {
		return err // rollback triggered by defer
	}

	return tx.Commit(ctx)
}

// WithTx executes a function inside a generic transaction.
func (s *Store) WithTx(ctx context.Context, fn func(context.Context, *Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)

	if err := fn(ctx, qtx); err != nil {
		return err // rollback triggered by defer
	}

	return tx.Commit(ctx)
}
