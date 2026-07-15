# Loom — Services & Infrastructure Setup Plan

Based on [Overview.md](./Overview.md), here is a complete inventory of every service and infrastructure component that must be set up, how they connect, and the order to stand them up.

---

## 1. Infrastructure Components (Set Up First)

These are shared infrastructure dependencies. All three application services depend on them, so they must be running before any service starts.

### 1a. PostgreSQL

| Detail | Value |
|---|---|
| **Purpose** | Persistent state for Trigger/API and Orchestration Engine |
| **Schemas** | `trigger_db` — workflows, webhook registrations, schedules, execution history records |
|  | `orchestration_db` — DAG execution state, per-node progress, branch decisions |
| **Node Worker Pool** | No dedicated schema (stateless; transient task state lives in-memory / queue) |
| **Local setup** | Single Postgres container in Docker Compose, two logical databases |
| **Prod setup** | AWS RDS — one instance with two databases, or two separate RDS instances for true isolation |

> **Important:** Each service owns its own schema exclusively. No cross-service direct SQL access. Cross-service reads go through the owning service's API.

### 1b. RabbitMQ

| Detail | Value |
|---|---|
| **Purpose** | Async task queues for all inter-service communication |
| **Queues needed** | `trigger-to-orchestration` — new workflow run requests |
|  | `orchestration-to-worker` — individual node execution tasks |
|  | `worker-to-orchestration` — node execution results (success/failure) |
|  | Dead Letter Exchanges (DLX) per queue — for retry/backoff on failed node executions |
| **Per-message TTL** | Configured on `orchestration-to-worker` for node timeout enforcement |
| **Local setup** | Single RabbitMQ container with management plugin enabled |
| **Prod setup** | Amazon MQ (RabbitMQ) or self-managed on EC2 |

### 1c. Redis

| Detail | Value |
|---|---|
| **Purpose** | Schedule/cron bookkeeping + lightweight caching (not a source of truth) |
| **Used by** | Trigger/API Service — cron tick tracking, deduplication, webhook rate-limit counters |
| **Local setup** | Single Redis container in Docker Compose |
| **Prod setup** | AWS ElastiCache (Redis) |

> **Note:** Redis is explicitly **not** a source of truth. If Redis is flushed, the system recovers from Postgres + RabbitMQ state. This simplifies disaster recovery.

---

## 2. Application Services (3 Go Services)

### 2a. Trigger/API Service (`trigger-api/`)

| Detail | Value |
|---|---|
| **Responsibility** | Workflow CRUD, webhook ingestion, schedule registration, execution-history API |
| **Scaling axis** | Inbound trigger volume (must be always-up, low-latency) |
| **Depends on** | PostgreSQL (`trigger_db`), RabbitMQ (publishes to `trigger-to-orchestration`), Redis (cron bookkeeping) |
| **Exposes** | REST API (external-facing): workflow CRUD, webhook endpoints, execution history queries |
| **Internal queue output** | Publishes "new run" messages to `trigger-to-orchestration` queue |

**Key internal packages:**
- `internal/webhooks` — webhook endpoint registration & ingestion
- `internal/schedules` — cron schedule management, Redis-backed tick tracking
- `internal/workflows` — workflow CRUD, JSON DAG validation

**Database migrations:** `trigger-api/migrations/` — versioned, independently deployed

---

### 2b. Orchestration Engine Service (`orchestration-engine/`)

| Detail | Value |
|---|---|
| **Responsibility** | DAG traversal, branch/condition evaluation, execution-state persistence, "what runs next" decisions |
| **Scaling axis** | Concurrent workflow count (consistency-critical, low CPU) |
| **Depends on** | PostgreSQL (`orchestration_db`), RabbitMQ (consumes from `trigger-to-orchestration` & `worker-to-orchestration`; publishes to `orchestration-to-worker`) |
| **Exposes** | Internal API only (used by Trigger/API Service for execution-state reads) |

**Key internal packages:**
- `internal/dag` — DAG data structure, topological traversal, next-node resolution
- `internal/state` — per-step execution state persistence (crash-safe)
- `internal/execution` — run lifecycle management, branch evaluation, retry policy enforcement

**Database migrations:** `orchestration-engine/migrations/` — versioned, independently deployed

> **Caution:** This is the consistency-critical "brain." A crash here must **never** lose or duplicate execution state. DAG progress is persisted per step to Postgres, not held in memory. Unacknowledged queue messages are redelivered on crash recovery.

---

### 2c. Node Worker Pool Service (`node-worker-pool/`)

| Detail | Value |
|---|---|
| **Responsibility** | Executes individual node actions pulled from a queue |
| **Scaling axis** | Node execution volume (horizontal scaling, stateless) |
| **Depends on** | RabbitMQ (consumes from `orchestration-to-worker`; publishes results to `worker-to-orchestration`) |
| **Stateless** | No database — transient task state only (in-memory during execution) |

**Key internal packages (one per node type):**
- `internal/httpnode` — HTTP request execution (the main bottleneck node type)
- `internal/transformnode` — data transformation / JSON manipulation
- `internal/delaynode` — delay/wait node (timer-based, not blocking a goroutine per delay)

**Node types supported (5 total):**

| Node Type | Handler | Notes |
|---|---|---|
| HTTP Request | `httpnode` | Outbound HTTP calls with configurable timeout, retry |
| Conditional Branch | Evaluated in Orchestration Engine | Not a worker task — branch logic runs in the engine |
| Delay/Wait | `delaynode` | Enqueues a delayed re-publish, not a blocking sleep |
| Data Transform | `transformnode` | JSON-to-JSON mapping/filtering |
| Webhook Trigger | Handled by Trigger/API Service | Entry point, not a worker task |

> **Note:** Only 3 of the 5 node types actually execute in the Worker Pool. Conditional branching is evaluated inside the Orchestration Engine (it's a routing decision, not an action). Webhook triggers are ingestion points handled by the Trigger/API Service.

---

## 3. Shared / Cross-Cutting Components

### 3a. Queue Contract Schemas (`shared/queue-contracts/`)

| Detail | Value |
|---|---|
| **Purpose** | Versioned message schemas shared across all three services |
| **Contains** | Go structs/types for: `NewRunMessage`, `NodeTaskMessage`, `NodeResultMessage` |
| **Used for** | Contract tests ensuring queue message compatibility across services |

### 3b. Service-to-Service Auth

| Detail | Value |
|---|---|
| **Mechanism** | Signed internal tokens (JWT or HMAC-signed) |
| **Where** | Orchestration Engine's internal API validates tokens from Trigger/API Service |
| **Shared secret** | Injected via environment variables, rotated per deployment |

### 3c. Database Migrations (Per-Service)

| Service | Migration path |
|---|---|
| Trigger/API | `trigger-api/migrations/` |
| Orchestration Engine | `orchestration-engine/migrations/` |
| Node Worker Pool | None (stateless) |

---

## 4. DevOps & Deployment Infrastructure

### 4a. Docker Compose (Local Development)

All components in a single `docker-compose.yml`:

| Container | Image |
|---|---|
| `postgres` | `postgres:16` (two databases: `trigger_db`, `orchestration_db`) |
| `rabbitmq` | `rabbitmq:3-management` |
| `redis` | `redis:7` |
| `trigger-api` | Built from `trigger-api/` |
| `orchestration-engine` | Built from `orchestration-engine/` |
| `node-worker-pool` | Built from `node-worker-pool/` (multiple replicas for load testing) |

### 4b. CI/CD — GitHub Actions

| Pipeline | Scope |
|---|---|
| `trigger-api.yml` | Build, test, deploy Trigger/API independently |
| `orchestration-engine.yml` | Build, test, deploy Orchestration Engine independently |
| `node-worker-pool.yml` | Build, test, deploy Node Worker Pool independently |

> **Tip:** Independent pipelines are the actual payoff of the microservice split — you can deploy a worker pool fix without touching the orchestration engine.

### 4c. AWS Production (Target)

| Component | AWS Service |
|---|---|
| 3 Go services | ECS Fargate (one task definition per service) or EC2 fleet |
| PostgreSQL | RDS (one or two instances) |
| RabbitMQ | Amazon MQ or self-managed |
| Redis | ElastiCache |
| Container registry | ECR |
| Load balancing | ALB in front of Trigger/API Service |

---

## 5. Testing Infrastructure

| Test Type | What It Validates | Infrastructure Needed |
|---|---|---|
| Unit tests | DAG traversal, branch evaluation, retry/backoff calc | None (pure logic) |
| Integration tests | Full trigger → orchestration → worker → result round trip | Docker Compose (all services + infra) |
| Contract tests | Queue message schema compatibility across services | Shared schema definitions |
| Failure-injection tests | Kill worker mid-execution → orchestration resumes correctly | Docker Compose + `docker kill` scripting |
| Load tests | Concurrent workflow runs, node throughput | Docker Compose or staging environment |

---

## 6. Observability (Recommended)

| Component | Purpose | Tool |
|---|---|---|
| Structured logging | Per-service JSON logs with correlation IDs | Go `slog` or `zerolog` |
| Distributed tracing | Full run-path visibility across all 3 services | OpenTelemetry |
| Metrics | Queue depth, node execution latency, error rates | Prometheus + Grafana |
| Health checks | Per-service `/healthz` endpoints | Built into each Go service |

---

## 7. Complete Service Dependency Map

```
                           ┌──────────────┐
           Webhooks ──────►│              │
                           │  Trigger/API │──── PostgreSQL (trigger_db)
           Cron ──────────►│   Service    │──── Redis
                           │              │
                           └──────┬───────┘
                                  │ publishes: NewRunMessage
                                  ▼
                           ┌──────────────┐
       ┌──────────────────►│ Orchestration│
       │  NodeResultMessage│    Engine    │──── PostgreSQL (orchestration_db)
       │                   │              │
       │                   └──────┬───────┘
       │                          │ publishes: NodeTaskMessage
       │                          ▼
       │                   ┌──────────────┐
       │                   │  Node Worker │
       └───────────────────│     Pool     │──── External APIs (outbound HTTP)
                           │              │
                           └──────────────┘

       Queue: RabbitMQ (all inter-service messages)
```

---

## 8. Summary — Total Component Count

| Category | Count | Components |
|---|---|---|
| **Infrastructure** | 3 | PostgreSQL, RabbitMQ, Redis |
| **Application Services** | 3 | Trigger/API, Orchestration Engine, Node Worker Pool |
| **Cross-Cutting** | 3 | Queue contracts, service auth, DB migrations |
| **DevOps** | 3 | Docker Compose, CI/CD pipelines, AWS deployment |
| **Total** | **12** | — |

---

## 9. Proposed Build Order

| Phase | What to Set Up | Rationale |
|---|---|---|
| **Phase 1** | Docker Compose + Postgres + RabbitMQ + Redis | Get infrastructure running locally |
| **Phase 2** | `shared/queue-contracts/` | Define message schemas before any service code |
| **Phase 3** | Trigger/API Service (CRUD + webhook ingestion) | Entry point — lets you create workflows and fire triggers |
| **Phase 4** | Orchestration Engine (DAG traversal + state persistence) | The "brain" — consumes trigger messages, produces node tasks |
| **Phase 5** | Node Worker Pool (HTTP node first, then transform, then delay) | Executes actual work — start with the most common node type |
| **Phase 6** | Integration & failure-injection tests | Validate the full round trip and crash recovery |
| **Phase 7** | CI/CD pipelines + AWS deployment | Production readiness |

---

## 10. Verification Plan

### Automated Tests
- `go test ./...` per service for unit tests
- Docker Compose-based integration test: trigger a workflow via webhook → verify full execution completes with correct per-node outputs in execution history
- Contract tests: validate queue message serialization/deserialization across service boundaries
- Failure test: `docker kill node-worker-pool` mid-execution → restart → verify workflow resumes without duplication

### Manual Verification
- Trigger a multi-node workflow via `curl` to the webhook endpoint
- Query execution history API to verify per-node input/output is recorded
- Verify cron-triggered workflows fire on schedule
- Test conditional branching with different input payloads to confirm correct path selection
