package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/loom/node-worker-pool/internal/db"
	"github.com/loom/shared/queue-contracts"
)

// IdempotentExecute wraps the execution of any node with atomic reserve-then-act idempotency.
func IdempotentExecute(
	ctx context.Context,
	store *db.Store,
	task contracts.NodeTaskMessage,
	executeFunc func(context.Context, json.RawMessage) (json.RawMessage, error),
) (json.RawMessage, bool, error) {
	// 1. Determine if this task requires idempotency (e.g. side-effecting nodes)
	// Right now we apply it to everything, but we could filter by node type if needed.
	
	// A simple pseudo-unique idempotency key.
	idempotencyKey := fmt.Sprintf("%s-%s", task.ExecutionID, task.NodeID)

	reserved, err := store.ReserveIdempotencyKey(ctx, task.ExecutionID, task.NodeID, idempotencyKey)
	if err != nil {
		return nil, false, fmt.Errorf("failed to reserve idempotency key: %w", err)
	}

	if !reserved {
		status, err := store.GetStatus(ctx, task.ExecutionID, task.NodeID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get status for unreserved key: %w", err)
		}

		if status == "COMPLETED" {
			log.Printf("Execution %s Node %s already COMPLETED. Skipping execution.", task.ExecutionID, task.NodeID)
			return []byte(`{"skipped": true, "reason": "already completed"}`), true, nil
		}

		if status == "IN_PROGRESS" {
			log.Printf("Execution %s Node %s is IN_PROGRESS from a previous failed attempt. Proceeding with residual risk.", task.ExecutionID, task.NodeID)
		}
	}

	// 2. Actually execute the wrapped node (HTTP, Email, etc.)
	output, err := executeFunc(ctx, task.Config)
	if err != nil {
		return output, false, err // Do not mark completed if it failed
	}

	// 3. Only on success, update status to COMPLETED
	if err := store.MarkCompleted(ctx, task.ExecutionID, task.NodeID); err != nil {
		log.Printf("CRITICAL: Failed to mark node %s in execution %s as COMPLETED: %v", task.NodeID, task.ExecutionID, err)
	}

	return output, false, nil
}
