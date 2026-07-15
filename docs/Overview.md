# Loom (Scoped n8n-Style Automation Backend)

## Purpose

A backend-focused workflow automation engine: users define workflows as a DAG of nodes (HTTP call, conditional, delay, transform, webhook trigger), triggered by webhooks or schedules, and the engine executes them reliably — with retries, crash-safe resumption, and full execution history. Deliberately scoped away from n8n's visual canvas and 400+ integrations to focus on the genuinely hard backend problem: reliable DAG execution.

## Problem It Solves

Full workflow-automation platforms (n8n, Zapier, Make) are multi-year products dominated by frontend canvas UI and integration breadth — neither of which demonstrates backend depth, and neither of which is feasible for one student to build well. This project isolates the actual distributed-systems problem underneath those products: given a workflow graph, execute each node exactly enough times, handle partial failures without losing state, support branching/conditional logic, and resume correctly if a worker crashes mid-execution.

## Core Features

- Workflows defined as JSON DAGs (nodes + edges), no visual builder required
- Triggers: webhook-based and schedule-based (cron)
- Node types: HTTP request, conditional branch, delay/wait, data transform, webhook trigger
- Per-node retry policy and timeout
- Crash-safe execution: workflow state persisted per step, resumable after a worker crash
- Full execution history with per-node input/output for debugging
- Queue-based node execution decoupled from the scheduling/orchestration logic

## Architecture — Microservice Split (3 services, each independently justified)

Each service exists because it has a **different scaling axis and failure profile** — not because "microservices" sounds good. This avoids the classic overengineering trap of a service per node type.

| Service | Responsibility | Why it's separate |
|---|---|---|
| **Trigger/API Service** | Workflow CRUD, webhook ingestion, schedule registration, execution-history API | Must be always-up, low-latency; scales with inbound trigger volume, independent of execution load |
| **Orchestration Engine Service** | DAG traversal, branch/condition evaluation, execution-state persistence, decides "what runs next" | The consistency-critical "brain" — low CPU, but must never lose or duplicate execution state; scales with concurrent workflow count, not node volume |
| **Node Worker Pool Service** | Executes individual node actions (HTTP calls, transforms, delays) pulled from a queue | The actual bottleneck — a slow external HTTP call must not block the scheduler or the API; scales horizontally with node execution volume independent of the other two |

**Communication**
- Trigger/API → Orchestration Engine: async via queue (a new workflow run is enqueued, not called synchronously)
- Orchestration Engine → Node Worker Pool: async via queue (per-node execution tasks)
- Node Worker Pool → Orchestration Engine: async result callback via queue (node result triggers next-step evaluation)
- All services share no direct database access to each other's tables — each owns its own schema; cross-service reads go through each service's API, not shared SQL

```
                 ┌─────────────────────┐
   Webhook/Cron ─►  Trigger/API Service │
                 └──────────┬──────────┘
                            │ enqueue: new run
                            ▼
                 ┌─────────────────────┐
                 │ Orchestration Engine │◄────────────┐
                 │ (DAG state machine)  │              │ node result
                 └──────────┬──────────┘              │
                            │ enqueue: node task       │
                            ▼                          │
                 ┌─────────────────────┐               │
                 │  Node Worker Pool    │───────────────┘
                 │ (HTTP/transform/etc)│
                 └─────────────────────┘

Each service: own Postgres schema. Queue: RabbitMQ (or Kafka if leaning into replay/audit of executions).
```

**Failure boundaries:** a crash in the Node Worker Pool cannot corrupt orchestration state — the Orchestration Engine only advances a workflow when it receives a node result message, so an unacknowledged task simply gets redelivered. A crash in the Orchestration Engine is recoverable because DAG progress is persisted per step, not held in memory.

**Honest trade-off to be ready to discuss:** this split adds real operational cost — three deployables, three schemas, network calls where a monolith would use function calls, and eventual consistency between services where a monolith would have transactional certainty. It's justified here because the three components' scaling needs genuinely diverge; it would **not** be justified if you were building this alone under a tight deadline and the workflows were low-volume — in that case, the monolith-with-worker-pool version (same internal boundaries, one deployable) is the more honest engineering call, and you should be able to argue *either* side in an interview.

## Tech Stack

**Backend**
- Go — all three services (goroutine-based worker pool for Node Worker Pool; DAG state machine for Orchestration Engine; HTTP API framework for Trigger/API)

**Database**
- PostgreSQL — one schema per service (`trigger_db`, `orchestration_db`, no persistent store needed in Node Worker Pool beyond transient task state)
- Redis — used only for schedule/cron bookkeeping and lightweight caching, not as a source of truth

**Infrastructure / Messaging**
- RabbitMQ — task queues between services (trigger→orchestration, orchestration→worker, worker→orchestration result), with per-message TTL + DLX for retry/backoff on failed node executions
- Docker / Docker Compose — local multi-service environment
- AWS — ECS (one service per task definition) or EC2 fleet; RDS for Postgres per service

**Testing**
- Unit tests: DAG traversal logic, branch evaluation, retry/backoff calculation
- Integration tests: full trigger → orchestration → worker → result round trip
- Contract tests: queue message schemas between the three services
- Failure-injection tests: kill Node Worker Pool mid-execution, verify orchestration resumes correctly without duplicating or losing steps
- Load tests: concurrent workflow runs, node-execution throughput

**Deployment**
- GitHub Actions CI/CD, one pipeline per service (independent build/deploy — this is the actual payoff of the microservice split)
- Per-service versioned DB migrations
- Service-to-service auth via signed internal tokens
- Independent rollback per service (tag-based, per service)

## Folder Structure (per-service repos or a monorepo with clear boundaries)

```
loom/
├── trigger-api/
│   ├── cmd/api/
│   ├── internal/{webhooks,schedules,workflows}
│   └── migrations/
├── orchestration-engine/
│   ├── cmd/engine/
│   ├── internal/{dag,state,execution}
│   └── migrations/
├── node-worker-pool/
│   ├── cmd/worker/
│   └── internal/{httpnode,transformnode,delaynode}
├── deploy/
│   ├── docker-compose.yml
│   └── aws/  # per-service ECS task defs
└── shared/
    └── queue-contracts/   # versioned message schemas shared across services
```

## Current Limitations & Possible Improvements

**Limitations**
- No visual builder — workflows are authored as JSON, which limits demoability to technical audiences
- Only 5 node types — nowhere near integration breadth of a real product
- No cross-workflow scheduling fairness (a burst of one workflow's runs can starve others in the Node Worker Pool queue)
- Eventual consistency between services means execution-history reads can lag actual state by the queue's processing delay

**Possible improvements**
- Add per-tenant queue partitioning for fair scheduling
- Add OpenTelemetry tracing across all three services for full run-path visibility
- Add a minimal read-only web UI to visualize a run's DAG and per-node status (still no drag-drop authoring)
- Kafka instead of RabbitMQ if execution history/replay-from-any-point becomes a requirement (explicitly not needed at this scope)