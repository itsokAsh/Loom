# Deep Project Context: Loom

## 1. Executive Project Summary
Loom is a backend-focused, microservice-based workflow automation engine inspired by tools like n8n and Zapier, but deliberately scoped away from visual canvas UIs. It focuses on the complex distributed-systems challenges of executing JSON-defined DAG (Directed Acyclic Graph) workflows reliably. It features built-in retries, crash-safe resumption via state machines, and decoupled execution through asynchronous messaging.

## 2. Workspace and Repository Structure
The project is structured as a monorepo containing three main Go services and shared libraries:
- `trigger-api/`: The entry point for API requests, webhook ingestion, and cron scheduling.
- `orchestration-engine/`: The state machine and "brain" managing DAG traversal and state persistence.
- `node-worker-pool/`: A stateless, horizontally scalable pool of workers that execute actual node logic (e.g., HTTP requests).
- `shared/`: Contains versioned message schemas (Queue Contracts) shared across all three services.
- `docs/`: Extensive project documentation and architecture designs.
- `scripts/`: Initialization scripts, such as database setups.
- `docker-compose.yml`: Local infrastructure orchestrator (Postgres, RabbitMQ, Redis, services).

## 3. Project Purpose
Loom solves the problem of reliable distributed DAG execution. It allows users to define workflows (nodes and edges) via JSON and trigger them via webhooks or cron schedules. The system guarantees that even if a worker or engine crashes mid-execution, state is not lost, and the workflow resumes correctly.

## 4. Technology Stack
- **Languages**: Go 1.23+
- **Database**: PostgreSQL 16 (with `sqlc` for typed queries), split into logical databases `trigger_db` and `orchestration_db`.
- **Messaging**: RabbitMQ (AMQP) for inter-service async communication.
- **Caching/State (Deprecated)**: Redis (redis:7) is running, but PostgreSQL has taken over its intended scheduling role.
- **Infrastructure**: Docker and Docker Compose for local development.

## 5. Intended Architecture
A strict 3-tier microservice architecture connected exclusively via RabbitMQ queues to enforce isolation and prevent cascading failures:
1. **Trigger/API** (Ingress): Handles HTTP API, webhooks, crons.
2. **Orchestration Engine** (State Machine): Traverses the DAG, evaluates conditions, and enqueues node tasks.
3. **Node Worker Pool** (Execution): Performs actual tasks (HTTP calls, JSON transforms, delays).

Each service operates on its own PostgreSQL schema (or is stateless) to ensure no direct cross-database dependencies.

## 6. Actual Architecture
The actual architecture strictly mirrors the intended architecture. The primary divergence is the handling of cron schedules: the documentation initially suggested Redis for cron tick tracking, but the current implementation uses a PostgreSQL `schedules` table with a lease mechanism. The outbox pattern is implemented in the `orchestration-engine` to guarantee RabbitMQ message delivery consistency alongside database state updates.

## 7. Service and Module Inventory

### Trigger/API Service
- **Purpose**: Entry point. Workflow CRUD, webhook ingestion, schedule polling.
- **Entry points**: `trigger-api/cmd/api/main.go`
- **Dependencies**: PostgreSQL (`trigger_db`), RabbitMQ (`trigger-to-orchestration` publisher, `orchestration-to-trigger-status` consumer).
- **Database Ownership**: `workflows`, `workflow_versions`, `webhooks`, `schedules`, `executions`.

### Orchestration Engine Service
- **Purpose**: The "brain" that traverses the workflow DAG and ensures crash-safe state persistence.
- **Entry points**: `orchestration-engine/cmd/engine/main.go`
- **Dependencies**: PostgreSQL (`orchestration_db`), RabbitMQ (Multiple queues).
- **Database Ownership**: `workflow_runs`, `node_executions`, `dispatched_tasks`, `outbox_messages`.

### Node Worker Pool
- **Purpose**: Horizontally scalable executors.
- **Entry points**: `node-worker-pool/cmd/worker/main.go`
- **Dependencies**: RabbitMQ.
- **Database Ownership**: None (stateless).
- **Important Notes**: Initially documented as missing in an earlier audit, it is currently implemented (including the `http` node executor).

## 8. Application Entry Points
- **API Server**: Starts via `go run trigger-api/cmd/api/main.go`, listens on port 8080.
- **Engine Server**: Starts via `go run orchestration-engine/cmd/engine/main.go`.
- **Worker Server**: Starts via `go run node-worker-pool/cmd/worker/main.go`.
- **Docker Compose**: `docker-compose up -d` boots all infrastructure, databases, migrations, and app containers simultaneously.

## 9. User Roles and Permissions
There is currently **no authentication or authorization** implemented.
- The `/v1` API endpoints are fully public.
- Webhook endpoints accept requests without verifying payload signatures (though plaintext `secret` values are generated in the database).
- Service-to-service internal authentication (signed tokens) mentioned in design docs is not yet implemented.

## 10. Major Workflow Traces

### E2E Execution Workflow
1. User triggers webhook -> `Trigger/API` (`POST /webhooks/{path}`).
2. `Trigger/API` persists `PENDING` execution in `trigger_db` and publishes `NewRunMessage` to `trigger-to-orchestration` RabbitMQ queue.
3. `Orchestration Engine` consumes `NewRunMessage`.
4. Engine initializes DAG state in `orchestration_db` (status `RUNNING`).
5. Engine determines next actionable nodes and publishes `NodeTaskMessage` to `orchestration-to-worker` via an Outbox pattern.
6. `Node Worker Pool` consumes `NodeTaskMessage`.
7. Worker executes logic (e.g., HTTP request to external API).
8. Worker publishes `NodeResultMessage` to `worker-to-orchestration`.
9. `Orchestration Engine` consumes `NodeResultMessage`, marks node completed, and evaluates the next DAG step.
10. Steps 5-9 repeat until DAG is exhausted.
11. Engine marks run `COMPLETED` and publishes `ExecutionStatusMessage` to `orchestration-to-trigger-status`.
12. `Trigger/API` consumes `ExecutionStatusMessage` and updates the final `execution` record in `trigger_db`.

## 11. API Inventory

| Method | Endpoint | Purpose | Implementation |
| ------ | -------- | ------- | -------------- |
| POST | `/v1/workflows` | Create a new workflow | Implemented |
| POST | `/v1/workflows/{id}/versions` | Add a version to a workflow | Implemented |
| GET | `/v1/workflows/{id}` | Get workflow details | Implemented |
| POST | `/v1/workflows/{id}/webhooks` | Generate webhook URL and secret | Implemented |
| POST | `/v1/workflows/{id}/schedules` | Generate cron schedule | Implemented |
| GET | `/v1/workflows/{id}/executions` | List executions | Implemented |
| GET | `/executions/{id}` | View specific execution status | Implemented |
| POST | `/webhooks/{path}` | Ingest payload and start execution | Implemented |

## 12. Database and Data Model Summary
- **trigger_db**:
  - `workflows`: Immutable definitions.
  - `workflow_versions`: Pinned DAG configurations.
  - `webhooks`: Webhook ingress paths and secrets.
  - `schedules`: Cron configurations and lease management.
  - `executions`: Top-level workflow invocation tracking.
- **orchestration_db**:
  - `workflow_runs`: Detailed internal DAG state for a run.
  - `node_executions`: Status and attempt tracking per DAG node.
  - `dispatched_tasks`: Timeout enforcement for running workers.
  - `outbox_messages`: Transactional outbox for RabbitMQ publishing.

## 13. Authentication and Authorization Flow
Not implemented. All APIs are currently unauthenticated. Webhooks do not verify signatures. 

## 14. External Integrations
- Node Worker Pool executes arbitrary HTTP requests to external endpoints (via the HTTP Node logic).
- Other planned nodes (Transform, Delay) are strictly internal logic.

## 15. Asynchronous Workflows
- Message Queues (RabbitMQ) serve as the backbone for moving state between isolated services.
- The Orchestration Engine utilizes a **Transactional Outbox** pattern (persisting outgoing messages to an `outbox_messages` table within the same transaction as state updates) to ensure 100% reliability in queue publishing.

## 16. Frontend and Mobile Structure
Explicitly out of scope for this backend orchestration project. 

## 17. Testing Structure
**0% Test Coverage.** 
- There are no `_test.go` files in the repository. This includes unit tests, integration tests, and E2E tests.

## 18. Infrastructure and Deployment
Currently driven exclusively by Docker Compose for local infrastructure.
- Uses `migrate/migrate` for database schema setups on boot.
- Requires `trigger_db` and `orchestration_db` initialized via `scripts/init-db.sh`.
- Deployment to cloud (AWS ECS) is planned in docs but no Terraform/CloudFormation code exists.

## 19. Configuration and Environment Variables
- `DATABASE_URL`: Required by `trigger-api` and `orchestration-engine`.
- `RABBITMQ_URL`: Required by all three services.

## 20. Major Dependencies
- `github.com/rabbitmq/amqp091-go`: RabbitMQ client.
- `github.com/jackc/pgx/v5`: PostgreSQL driver.
- `github.com/go-chi/chi/v5`: HTTP Router.
- `github.com/robfig/cron/v3`: Cron expression parser.

## 21. Current Implementation State
The core happy-path execution loop is mostly assembled. The `node-worker-pool` has an HTTP node executor, the DAG state machine can traverse nodes, and the `trigger-api` can start workflows and update statuses.
However:
- The system is completely lacking tests.
- Authentication and security features are omitted.
- The `dag.Evaluator` does not actually evaluate branching conditionals (it is stubbed out).
- Idempotency bugs exist in webhook ingestion.

## 22. Documentation-Code Discrepancies
- **Critical Discrepancy**: `COMPREHENSIVE_PROJECT_AUDIT.md` and `SERVICE_FEATURE_AND_OVERVIEW_VERIFICATION.md` declare that the `node-worker-pool` and the Trigger API's `ExecutionStatusMessage` consumer are completely missing. **This is false.** The `node-worker-pool` codebase exists (with an HTTP node), and `trigger-api/cmd/api/main.go` actively spins up a consumer for `orchestration-to-trigger-status`.
- Redis is declared as the cron engine in `Overview.md`, but `schedules` in PostgreSQL is actually used for leasing and tick deduplication.

## 23. Important Files and Directories
- `trigger-api/cmd/api/main.go`
- `orchestration-engine/internal/engine/orchestrator.go`
- `node-worker-pool/internal/worker/rabbitmq.go`
- `shared/queue-contracts/events.go`
- `docker-compose.yml`

## 24. Areas Requiring Deeper Investigation
- **DAG Condition Evaluator**: Needs deep dive into how `orchestration-engine/internal/dag/evaluator.go` currently stubs conditional branch logic.
- **Webhook Idempotency and Security**: Need to investigate how to implement HMAC verification and fix the duplicate-run on retry issue in `trigger-api/internal/webhooks/handler.go`.
- **Outbox Poller**: Need to check if `outbox_relay.go` is actively running and properly acknowledging RabbitMQ errors.

---

# Session Context Summary

**Project Purpose:** Loom is a backend-focused workflow execution engine (like n8n) guaranteeing crash-safe DAG execution using Postgres and RabbitMQ.
**Architecture:** 3 independent Go microservices (`trigger-api`, `orchestration-engine`, `node-worker-pool`) communicating via RabbitMQ. State is kept in separate PostgreSQL schemas.
**Important Services:** 
- `trigger-api` (ingress, CRUD, scheduling, status listener)
- `orchestration-engine` (DAG state machine, state persistence)
- `node-worker-pool` (scalable workers executing HTTP/Transform nodes)
**Major Workflows:** Webhook/Cron -> Trigger/API -> (RMQ) -> Orchestration Engine -> (RMQ Outbox) -> Node Worker -> (RMQ) -> Orchestration Engine -> (RMQ Outbox) -> Trigger/API.
**Important Database Entities:** `trigger_db` (workflows, webhooks, schedules, executions). `orchestration_db` (workflow_runs, node_executions, outbox_messages).
**Authentication Model:** Completely unauthenticated currently.
**Current Focus:** Closing the loop was recently completed (worker pool and status consumer exist now). Current gaps: zero test coverage, stubbed DAG branching logic, webhook security, and idempotency bugs.
**Important Constraints:** Services MUST remain isolated. No direct cross-service database access. Communication MUST flow through RabbitMQ contracts (`shared/queue-contracts`).
**Commands required to run the project:** `docker-compose up -d` starts all infrastructure, migrations, and app containers.
