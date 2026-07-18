# Loom System Audit Log

**Date:** 2026-07-18
**Auditor:** AntiGravity
**Scope:** Trigger API, Orchestration Engine (Milestone 1), Shared Contracts, Architecture

## 1. Architectural Alignment
- **Microservices Pattern**: The project correctly adheres to the 3-service decoupling (Trigger API, Orchestration Engine, Node Worker Pool). 
- **Data Isolation**: `trigger_db` and `orchestration_db` are properly segregated.
- **Asynchronous Communication**: RabbitMQ safely brokers all inter-service communications using four dedicated queues.

## 2. Shared Contracts (Queue Contracts)
- **Status**: **PASS**
- **Findings**: The `events.go` file correctly implements the V2 specifications. JSON tags are properly set to `snake_case`. `DAGDefinition` is successfully encapsulated as `json.RawMessage` within `NewRunMessage` to ensure the exact runtime snapshot of the DAG is persisted in the orchestration engine, preventing retrospective workflow mutation bugs.

## 3. Trigger API Service
- **Status**: **PASS**
- **Findings**: 
  - `webhooks` and `schedules` modules correctly handle inbound triggers.
  - Successfully patched to produce the V2 `NewRunMessage` (resolving the previous `DAGDefinition` structural conflicts).
  - Validation ensures only valid JSON DAGs are enqueued.

## 4. Orchestration Engine (Milestone 1)
- **Status**: **PASS (Vertical Slice Complete)**
- **Findings**:
  - **Database & State**: Schema contains `workflow_runs`, `node_executions`, and `dispatched_tasks`. Idempotency is enforced using `ON CONFLICT DO NOTHING`.
  - **Concurrency Safety**: Crucial per-run row locking implemented successfully in `internal/db/store.go` via `WithRunLock` (`SELECT ... FOR UPDATE`). This guarantees that concurrent node results cannot cause a race condition leading to double-dispatch.
  - **Message Handling**: Idempotent consumption of `NewRunMessage` and `NodeResultMessage` is in place with manual ACK/NACK behaviors on RabbitMQ.
  - **DAG Traversal**: A stub DAG evaluator correctly traverses linear DAGs, recognizing parent node success before dispatching the next nodes.

## 5. Security & Fault Tolerance
- **Database Transaction Safety**: High. All critical state transitions in the Orchestration Engine are wrapped in locked transactions.
- **Message Durability**: RabbitMQ queues are declared durable. Publisher confirms are enabled to ensure tasks are not silently dropped before they reach the broker.

## 6. Action Items & Pending Milestones
- **Node Worker Pool**: A stub or real implementation is required to fully complete end-to-end testing of the linear DAG traversal.
- **Milestone 2 (Retries & Sweeper)**: The `max_attempts` field is populated, but DLX-based retry backoff logic is pending. The background reconciliation sweep to catch `dispatched_tasks` that exceed `attempt_timeout_at` must be built.
- **Milestone 3 (Expr Engine)**: The DAG evaluator (`internal/dag/evaluator.go`) currently bypasses conditional expression compilation (`expr-lang/expr`) and returns raw configs. This must be wired up.
