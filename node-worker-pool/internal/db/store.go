package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// ReserveIdempotencyKey attempts to insert an IN_PROGRESS record for a given execution ID and node ID.
// Returns true if the reservation was successfully acquired.
// Returns false if a record already exists (meaning another worker reserved or completed it).
func (s *Store) ReserveIdempotencyKey(ctx context.Context, executionID, nodeID, idempotencyKey string) (bool, error) {
	query := `
		INSERT INTO external_call_log (execution_id, node_id, idempotency_key, status)
		VALUES ($1, $2, $3, 'IN_PROGRESS')
		ON CONFLICT (execution_id, node_id) DO NOTHING
	`

	cmdTag, err := s.pool.Exec(ctx, query, executionID, nodeID, idempotencyKey)
	if err != nil {
		return false, fmt.Errorf("failed to execute reservation query: %w", err)
	}

	return cmdTag.RowsAffected() == 1, nil
}

// GetStatus returns the current status of the external call log for the given execution ID and node ID.
func (s *Store) GetStatus(ctx context.Context, executionID, nodeID string) (string, error) {
	query := `
		SELECT status FROM external_call_log 
		WHERE execution_id = $1 AND node_id = $2
	`
	var status string
	err := s.pool.QueryRow(ctx, query, executionID, nodeID).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	return status, nil
}

// MarkCompleted updates the status of an external call log to 'COMPLETED'.
func (s *Store) MarkCompleted(ctx context.Context, executionID, nodeID string) error {
	query := `
		UPDATE external_call_log 
		SET status = 'COMPLETED', updated_at = CURRENT_TIMESTAMP
		WHERE execution_id = $1 AND node_id = $2
	`

	_, err := s.pool.Exec(ctx, query, executionID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to mark as completed: %w", err)
	}

	return nil
}
