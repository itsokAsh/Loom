package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	contracts "github.com/loom/shared/queue-contracts"
	"github.com/loom/orchestration-engine/internal/dag"
	"github.com/loom/orchestration-engine/internal/db"
	"github.com/loom/orchestration-engine/internal/queue"
)

type Orchestrator struct {
	store     *db.Store
	rmq       *queue.RabbitMQ
	evaluator *dag.Evaluator
}

func NewOrchestrator(store *db.Store, rmq *queue.RabbitMQ, evaluator *dag.Evaluator) *Orchestrator {
	return &Orchestrator{
		store:     store,
		rmq:       rmq,
		evaluator: evaluator,
	}
}

func (o *Orchestrator) HandleNewRun(msg contracts.NewRunMessage) error {
	ctx := context.Background()
	var execID pgtype.UUID
	if err := execID.Scan(msg.ExecutionID); err != nil {
		return fmt.Errorf("invalid execution ID: %w", err)
	}

	var wfID pgtype.UUID
	if err := wfID.Scan(msg.WorkflowID); err != nil {
		return fmt.Errorf("invalid workflow ID: %w", err)
	}

	err := o.store.InsertWorkflowRun(ctx, db.InsertWorkflowRunParams{
		ExecutionID:     execID,
		WorkflowID:      wfID,
		WorkflowVersion: int32(msg.WorkflowVersion),
		DagDefinition:   msg.DAGDefinition,
		Status:          "RUNNING",
		StartedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to insert workflow run: %w", err)
	}

	var actionableNodes []contracts.Node
	var toPublish []contracts.NodeTaskMessage

	err = o.store.WithRunLock(ctx, execID, func(ctx context.Context, qtx *db.Queries) error {
		nodeStates := make(map[string]string)

		nodes, err := o.evaluator.NextActionableNodes(msg.DAGDefinition, nodeStates)
		if err != nil {
			return err
		}
		actionableNodes = nodes

		for _, n := range actionableNodes {
			err := qtx.InsertNodeExecution(ctx, db.InsertNodeExecutionParams{
				ExecutionID: execID,
				NodeID:      n.ID,
				Status:      "QUEUED",
				MaxAttempts: 3, 
			})
			if err != nil {
				return fmt.Errorf("failed to insert node execution for %v: %w", n.ID, err)
			}

			timeoutAt := time.Now().Add(5 * time.Minute)
			dispatchID, err := qtx.InsertDispatchedTask(ctx, db.InsertDispatchedTaskParams{
				ExecutionID:      execID,
				NodeID:           n.ID,
				AttemptTimeoutAt: pgtype.Timestamptz{Time: timeoutAt, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to insert dispatched task for %v: %w", n.ID, err)
			}

			configBytes, _ := o.evaluator.EvaluateConfig(n.Config, nil)

			toPublish = append(toPublish, contracts.NodeTaskMessage{
				ExecutionID:  msg.ExecutionID,
				NodeID:       n.ID,
				DispatchID:   fmt.Sprintf("%x", dispatchID.Bytes),
				AttemptCount: 1,
				NodeType:     n.Type,
				Config:       configBytes,
			})
		}

		if len(actionableNodes) == 0 {
			if err := qtx.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
				ExecutionID: execID,
				Status:      "FAILED",
				CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}); err != nil {
				return fmt.Errorf("failed to update run status: %w", err)
			}
		}

		for _, task := range toPublish {
			body, _ := json.Marshal(task)
			if err := qtx.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
				Queue:   "orchestration-to-worker",
				Payload: body,
			}); err != nil {
				return fmt.Errorf("failed to insert task to outbox: %w", err)
			}
		}

		if len(actionableNodes) == 0 {
			t := time.Now()
			statusMsg := contracts.ExecutionStatusMessage{
				ExecutionID: msg.ExecutionID,
				Status:      "FAILED",
				UpdatedAt:   t,
				CompletedAt: &t,
			}
			body, _ := json.Marshal(statusMsg)
			if err := qtx.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
				Queue:   "orchestration-to-trigger-status",
				Payload: body,
			}); err != nil {
				return fmt.Errorf("failed to insert status to outbox: %w", err)
			}
		}

		return nil
	})

	return err
}

func (o *Orchestrator) HandleNodeResult(msg contracts.NodeResultMessage) error {
	ctx := context.Background()
	var execID pgtype.UUID
	if err := execID.Scan(msg.ExecutionID); err != nil {
		return err
	}

	var toPublish []contracts.NodeTaskMessage
	runCompleted := false

	err := o.store.WithRunLock(ctx, execID, func(ctx context.Context, qtx *db.Queries) error {
		var dispatchUUID pgtype.UUID
		if err := dispatchUUID.Scan(msg.DispatchID); err != nil {
			return err
		}

		dt, err := qtx.GetDispatchedTask(ctx, db.GetDispatchedTaskParams{
			ExecutionID: execID,
			NodeID:      msg.NodeID,
		})
		if err != nil {
			return nil 
		}
		if dt.DispatchID != dispatchUUID {
			return nil
		}

		err = qtx.DeleteDispatchedTask(ctx, db.DeleteDispatchedTaskParams{
			ExecutionID: execID,
			NodeID:      msg.NodeID,
		})
		if err != nil {
			return fmt.Errorf("failed to delete dispatched task: %w", err)
		}

		err = qtx.UpdateNodeExecutionStatus(ctx, db.UpdateNodeExecutionStatusParams{
			ExecutionID:  execID,
			NodeID:       msg.NodeID,
			Status:       msg.Status,
			OutputData:   msg.OutputData,
			ErrorMessage: pgtype.Text{String: msg.ErrorMessage, Valid: msg.ErrorMessage != ""},
			CompletedAt:  pgtype.Timestamptz{Time: msg.CompletedAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to update node execution: %w", err)
		}

		run, err := qtx.GetWorkflowRun(ctx, execID)
		if err != nil {
			return err
		}

		nodeExs, err := qtx.ListAllNodeExecutions(ctx, execID)
		if err != nil {
			return err
		}
		
		nodeStates := make(map[string]string)
		for _, n := range nodeExs {
			nodeStates[n.NodeID] = n.Status
		}

		nodes, err := o.evaluator.NextActionableNodes(run.DagDefinition, nodeStates)
		if err != nil {
			return err
		}

		if len(nodes) == 0 {
			// Check if any node is pending/queued/running
			anyRunning := false
			for _, st := range nodeStates {
				if st == "PENDING" || st == "QUEUED" || st == "RUNNING" {
					anyRunning = true
					break
				}
			}

			if !anyRunning {
				runCompleted = true
				if err := qtx.UpdateWorkflowRunStatus(ctx, db.UpdateWorkflowRunStatusParams{
					ExecutionID: execID,
					Status:      "COMPLETED",
					CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}); err != nil {
					return fmt.Errorf("failed to update run status: %w", err)
				}
			}
		} else {
			for _, n := range nodes {
				err := qtx.InsertNodeExecution(ctx, db.InsertNodeExecutionParams{
					ExecutionID: execID,
					NodeID:      n.ID,
					Status:      "QUEUED",
					MaxAttempts: 3, 
				})
				if err != nil {
					return err
				}

				timeoutAt := time.Now().Add(5 * time.Minute)
				dispatchID, err := qtx.InsertDispatchedTask(ctx, db.InsertDispatchedTaskParams{
					ExecutionID:      execID,
					NodeID:           n.ID,
					AttemptTimeoutAt: pgtype.Timestamptz{Time: timeoutAt, Valid: true},
				})
				if err != nil {
					return err
				}

				configBytes, _ := o.evaluator.EvaluateConfig(n.Config, nil)

				toPublish = append(toPublish, contracts.NodeTaskMessage{
					ExecutionID:  msg.ExecutionID,
					NodeID:       n.ID,
					DispatchID:   fmt.Sprintf("%x", dispatchID.Bytes),
					AttemptCount: 1,
					NodeType:     n.Type,
					Config:       configBytes,
				})
			}
		}

		for _, task := range toPublish {
			body, _ := json.Marshal(task)
			if err := qtx.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
				Queue:   "orchestration-to-worker",
				Payload: body,
			}); err != nil {
				return fmt.Errorf("failed to insert task to outbox: %w", err)
			}
		}

		if runCompleted {
			t := time.Now()
			statusMsg := contracts.ExecutionStatusMessage{
				ExecutionID: msg.ExecutionID,
				Status:      "COMPLETED",
				UpdatedAt:   t,
				CompletedAt: &t,
			}
			body, _ := json.Marshal(statusMsg)
			if err := qtx.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
				Queue:   "orchestration-to-trigger-status",
				Payload: body,
			}); err != nil {
				return fmt.Errorf("failed to insert status to outbox: %w", err)
			}
		}

		return nil
	})

	return err
}
