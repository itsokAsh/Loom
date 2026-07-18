# Comprehensive Project Audit and Architecture Gap Analysis

## A. Executive Summary
The Loom project has established a foundation for its asynchronous, microservice-based workflow engine. The core abstractions (DAG parsing, separated databases, queue-based messaging via an outbox pattern) are partially present. However, the system is **not production-ready** and cannot currently execute workflows end-to-end. 

The most critical gap is the **complete absence of the Node Worker Pool service**, meaning no workflow tasks can actually be executed. Furthermore, the `trigger-api` lacks a queue consumer to update execution status, resulting in all executions permanently remaining in the `PENDING` state. Security vulnerabilities exist (plaintext secrets, lack of webhook verification), and there is a total absence of automated tests. The most important next step is to close the execution loop by building the Node Worker Pool and the Trigger API status consumer.

## B. Architecture Completion Score
**Overall Completion:** ~35%

* **Implementation (30%):** Trigger API is mostly implemented for CRUD operations. Orchestration Engine is partially implemented but lacks condition evaluation. Node Worker Pool is 0% implemented.
* **Integration (40%):** RabbitMQ publishing works from Trigger API to Orchestration Engine, and Orchestration to (missing) Workers, but the return loop to Trigger API is missing.
* **Testing (0%):** Zero unit, integration, or end-to-end tests exist in the repository.
* **Security (10%):** Basic CORS/auth is missing. Webhook secrets are stored in plaintext and not verified.
* **Deployment Readiness (25%):** Docker-compose exists for local infrastructure, but CI/CD pipelines, production Dockerfiles, and AWS deployment configurations are missing.

## C. Critical Issues
* **Blocker (Workflow Failure):** The `node-worker-pool` service is completely missing. Workflows get queued but never execute.
* **Blocker (Invalid Application State):** The `trigger-api` does not consume the `orchestration-to-trigger-status` queue. Execution status in `trigger_db` will remain `PENDING` forever.
* **Blocker (Invalid Application State):** `HandleIncomingWebhook` publishes a duplicate `NewRunMessage` to RabbitMQ if an execution with the same idempotency key already exists, breaking idempotency guarantees.
* **Blocker (Invalid Application State):** The `dag.Evaluator` does not evaluate edge conditions (`edge.Condition`). Branching logic is non-functional.
* **Blocker (Invalid Application State):** DAG Join Deadlocks. If a DAG node has multiple parents, the current evaluator requires all parents to be `SUCCESS` or `SKIPPED`. If a condition routes execution away from one parent, that parent will never execute, causing the join node to wait indefinitely.
* **Security Compromise:** Webhook secrets are generated and stored in plaintext in the database, and inbound webhook requests are not cryptographically verified against this secret.
* **Security Compromise:** The `/v1` API routes in `trigger-api` are completely unauthenticated.

## D. Detailed Findings

### Architecture
* **Drift:** The architecture (`Overview.md`) states Redis is used for schedule/cron bookkeeping. The implementation (`trigger-api/migrations/000001_init_schema.up.sql`) abandons Redis in favor of a PostgreSQL `schedules` table with a lease mechanism. This drift needs reconciliation.
* **Missing Service:** The `Node Worker Pool` service is completely missing from the codebase.
* **Missing Loop:** `trigger-api` lacks a consumer to listen for workflow completion events.

### Functional Bugs
* **Duplicate Execution Publishing:** In `trigger-api/internal/webhooks/handler.go`, if `CreateExecution` fails due to a conflict, the execution is fetched and the run message is published *again*, leading to duplicate orchestration runs.
* **Incomplete DAG Evaluator:** `orchestration-engine/internal/dag/evaluator.go` ignores `edge.Condition` and implements `EvaluateConfig` as a stub that does not interpolate expressions.
* **Lease Expiration Unhandled:** In `trigger-api/internal/schedules/poller.go`, the query specifically checks `leased_by IS NULL` but ignores rows where the lease has expired (`lease_expires_at < now()`). Crashed pollers leave schedules dead.

### APIs
| Endpoint | Expected Contract | Actual Contract | Consumers | Problem | Severity | Required Fix |
| -------- | ----------------- | --------------- | --------- | ------- | -------- | ------------ |
| `GET /v1/workflows` | List workflows with pagination | Missing | Frontend UI | Cannot discover workflows | High | Implement endpoint |
| `PUT /v1/workflows/{id}` | Update workflow details | Missing | Frontend UI | Cannot rename/update workflows | Medium | Implement endpoint |
| `GET /v1/workflows/{id}/executions` | Paginated executions | Hardcoded limit=20, no cursor support | Frontend UI | Incomplete pagination | Low | Parse cursor/limit from query params |

### Database
* **Plaintext Secrets:** `trigger_db.webhooks.secret` is stored in plaintext. It should be hashed.
* **Missing Index:** The `outbox_messages` table in `orchestration-engine` has an index on `created_at` where `published_at IS NULL`, but polling might require ordering by `id` or using `SKIP LOCKED`.

### Integrations
* **RabbitMQ Dead Letters:** Dead Letter Exchanges (DLX) are specified in the architecture for node retry logic but are not declared in the RabbitMQ queue setup in the code.
* **Status Queue Abandoned:** The `orchestration-to-trigger-status` queue receives messages from the engine but has no consumer in `trigger-api`.

### Security
* **Unauthenticated API:** The Trigger API exposes workflow and execution CRUD operations without any authentication middleware.
* **Missing Webhook Verification:** `HandleIncomingWebhook` looks up the webhook by path but ignores the payload signature/secret entirely, allowing trivial spoofing.

### Reliability
* **Cron Lease Expiration:** In `schedules/poller.go`, there is no logic to recover expired leases.
* **Missing Retries on Outbox:** The outbox relay in `orchestration-engine` does not implement backoff for RabbitMQ connection failures.

### Testing
* **Zero Tests:** There are no `.go` test files in the entire repository. Unit tests, integration tests, and contract tests are completely missing.

### Deployment
* **Missing CI/CD:** GitHub Actions workflows mentioned in the architecture do not exist.
* **Docker Configuration:** `docker-compose.yml` mounts a local script for DB init, but production deployment configurations (e.g., AWS ECS Task Definitions, Terraform) are missing.

## E. Missing Architectural Requirements
* **Node Worker Pool:** Missing entirely.
* **Queue Dead Letter Exchanges:** Not implemented.
* **Service-to-Service Auth:** Signed internal tokens are missing.
* **DAG Conditional Evaluation:** Implemented incorrectly (stubbed).
* **Execution Status Updates:** Not integrated (missing consumer in Trigger API).
* **Automated Tests:** Missing entirely.
* **CI/CD Pipelines:** Missing entirely.

## F. End-to-End Workflow Status
* **Webhook Ingestion:** Partially Working (Accepts payload, persists to DB, queues message. Fails to verify HMAC. Fails on idempotency duplicates).
* **Cron Triggering:** Partially Working (Polls DB, queues message. Fails to reclaim expired leases).
* **DAG Orchestration:** Broken (Evaluates next nodes, queues tasks, but conditions/interpolation fail).
* **Node Execution:** Missing (No worker pool exists to consume tasks).
* **Execution Status Tracking:** Broken (Engine publishes status, but Trigger API never consumes it).

## G. Prioritized Remediation Plan

### P0 — Immediate Blockers
1. **Implement Node Worker Pool:**
   - **Reason:** Workflows cannot execute without workers.
   - **Affected files:** `node-worker-pool/*` (New Service)
   - **Complexity:** Large
2. **Implement Trigger API Status Consumer:**
   - **Reason:** Execution statuses remain stuck in `PENDING`.
   - **Affected files:** `trigger-api/cmd/api/main.go`, `trigger-api/internal/queue/consumer.go`
   - **Complexity:** Medium
3. **Fix Idempotency Duplicate Publishing:**
   - **Reason:** Duplicate runs are enqueued on retries.
   - **Affected files:** `trigger-api/internal/webhooks/handler.go`
   - **Complexity:** Small

### P1 — Required before production
1. **Implement DAG Condition Evaluation & Config Interpolation:**
   - **Reason:** Workflows cannot branch or use dynamic data.
   - **Affected files:** `orchestration-engine/internal/dag/evaluator.go`
   - **Complexity:** Medium
2. **Implement Webhook HMAC Verification:**
   - **Reason:** Anyone can trigger a webhook if they guess/find the URL.
   - **Affected files:** `trigger-api/internal/webhooks/handler.go`
   - **Complexity:** Small
3. **Add Comprehensive Test Suite:**
   - **Reason:** 0% test coverage guarantees regressions.
   - **Affected files:** `*_test.go` across all packages
   - **Complexity:** Large

### P2 — Important stability and maintainability work
1. **Configure RabbitMQ DLX & Retry Policies:**
   - **Reason:** Transient HTTP errors in nodes will cause workflow failures instead of retries.
   - **Affected files:** `orchestration-engine/internal/queue/rabbitmq.go`
   - **Complexity:** Medium
2. **Fix Expired Lease Recovery in Poller:**
   - **Reason:** Crashed pollers leave cron schedules stuck forever.
   - **Affected files:** `trigger-api/internal/schedules/poller.go`, `query.sql`
   - **Complexity:** Small

### P3 — Improvements and technical debt
1. **Implement API Pagination & Missing Routes:**
   - **Reason:** Poor UX for frontends.
   - **Affected files:** `trigger-api/internal/workflows/handler.go`
   - **Complexity:** Small
2. **Reconcile Architecture Docs vs. Implementation:**
   - **Reason:** Redis drift causes confusion.
   - **Affected files:** `docs/Overview.md`
   - **Complexity:** Small

## H. Recommended Implementation Milestones

### Milestone 1: Close the Execution Loop
* **Objective:** Get a simple HTTP node workflow to execute end-to-end and update its status to COMPLETED.
* **Scope:** Node Worker Pool (HTTP only), Trigger API Status Consumer, Idempotency fix.
* **Completion criteria:** A webhook trigger successfully runs an HTTP node, and `GET /v1/executions/{id}` returns `COMPLETED`.

### Milestone 2: Feature Completeness & DAG Logic
* **Objective:** Support branching, variables, and delays.
* **Scope:** Evaluator condition logic, Config interpolation, Delay/Transform node workers.
* **Completion criteria:** A workflow with a conditional branch correctly routes execution based on webhook payload data.

### Milestone 3: Security & Stability
* **Objective:** Make the system production-safe.
* **Scope:** Webhook verification, API Auth, RabbitMQ DLX, Expired lease recovery, Test Suite.
* **Completion criteria:** 80% test coverage, unauthorized requests rejected, failed HTTP nodes retry via DLX.
