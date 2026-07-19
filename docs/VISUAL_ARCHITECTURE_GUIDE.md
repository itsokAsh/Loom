# Loom Visual Architecture Guide

## 1. THE BIG PICTURE: 3 Core Services

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         LOOM WORKFLOW ENGINE                             │
└─────────────────────────────────────────────────────────────────────────┘

┌──────────────────────┐        ┌──────────────────────┐        ┌──────────────────────┐
│   TRIGGER/API        │        │  ORCHESTRATION       │        │   NODE WORKER        │
│     SERVICE          │        │     ENGINE           │        │      POOL            │
│                      │        │                      │        │                      │
│  Entry Point         │        │  The "Brain"         │        │  Task Executor       │
│  - Create workflows  │        │  - DAG traversal     │        │  - HTTP calls        │
│  - Receive webhooks  │        │  - State management  │        │  - Transforms        │
│  - Manage schedules  │        │  - Condition logic   │        │  - Delays            │
│  - Show history      │        │  - Retry decisions   │        │  - Email sending     │
│                      │        │                      │        │                      │
│  Database:           │        │  Database:           │        │  Database:           │
│  trigger_db          │        │  orchestration_db    │        │  NONE (stateless)    │
│                      │        │                      │        │                      │
│  Port: 8080          │        │  (Internal only)     │        │  (Scales 1-100+)     │
└──────────┬───────────┘        └──────────┬───────────┘        └──────────┬───────────┘
           │                               │                               │
           │                               │                               │
           └───────────────┬───────────────┴───────────────┬───────────────┘
                           │                               │
                           │      RabbitMQ Queues         │
                           │   (Async Communication)       │
                           └───────────────────────────────┘

                     ┌─────────────────────────────────┐
                     │     SHARED INFRASTRUCTURE       │
                     ├─────────────────────────────────┤
                     │  PostgreSQL (2 databases)       │
                     │  RabbitMQ (3 queues)            │
                     │  Redis (cron bookkeeping)       │
                     └─────────────────────────────────┘
```

---

## 2. PREDEFINED NODE TYPES (Building Blocks)

### These are the "LEGO pieces" users can configure:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    LOOM NODE TYPE CATALOG                                │
│                  (What Users Can Put in Workflows)                       │
└─────────────────────────────────────────────────────────────────────────┘

┌──────────────────┐
│   1. HTTP NODE   │  ← Makes API calls to external services
├──────────────────┤
│ Config:          │
│ - url            │  "https://api.stripe.com/charges"
│ - method         │  "POST" / "GET" / "PUT" / "DELETE"
│ - headers        │  {"Authorization": "Bearer xxx"}
│ - body           │  {"amount": 1000}
│ - timeout        │  30 seconds
└──────────────────┘

┌──────────────────┐
│ 2. CONDITIONAL   │  ← Branches workflow based on conditions
├──────────────────┤
│ Config:          │
│ - condition      │  "{{node1.output.status}} == 'success'"
│ - true_path      │  node_id to execute if true
│ - false_path     │  node_id to execute if false
└──────────────────┘

┌──────────────────┐
│ 3. TRANSFORM     │  ← Manipulates data (JSON transformation)
├──────────────────┤
│ Config:          │
│ - script         │  JavaScript-like expression
│ - mapping        │  {"newField": "{{input.oldField}}"}
└──────────────────┘

┌──────────────────┐
│  4. DELAY NODE   │  ← Waits before continuing
├──────────────────┤
│ Config:          │
│ - duration       │  "5m" / "1h" / "30s"
│ - unit           │  "seconds" / "minutes" / "hours"
└──────────────────┘

┌──────────────────┐
│ 5. EMAIL NODE    │  ← Sends emails
├──────────────────┤
│ Config:          │
│ - provider       │  "sendgrid" / "ses" / "smtp"
│ - to             │  "user@example.com"
│ - subject        │  "Welcome!"
│ - body           │  "Thanks for signing up"
│ - api_key        │  "SG.xxx..."
└──────────────────┘

┌──────────────────┐
│ 6. WEBHOOK       │  ← Trigger point (entry to workflow)
├──────────────────┤
│ Config:          │
│ - path           │  "/webhooks/abc123"
│ - secret         │  (For HMAC verification)
│ - method         │  "POST"
└──────────────────┘

┌──────────────────┐
│ 7. SCHEDULE      │  ← Cron-based trigger
├──────────────────┤
│ Config:          │
│ - cron           │  "0 9 * * *" (daily at 9am)
│ - timezone       │  "America/New_York"
└──────────────────┘

┌──────────────────┐
│ 8. DATABASE      │  ← Query/insert into databases
├──────────────────┤
│ Config:          │
│ - type           │  "postgres" / "mysql" / "mongodb"
│ - query          │  "INSERT INTO users ..."
│ - connection     │  "postgres://..."
└──────────────────┘

┌──────────────────┐
│ 9. SLACK NODE    │  ← Post to Slack channels
├──────────────────┤
│ Config:          │
│ - token          │  "xoxb-..."
│ - channel        │  "#alerts"
│ - message        │  "Deployment complete!"
└──────────────────┘

┌──────────────────┐
│ 10. FILTER       │  ← Filters/validates data
├──────────────────┤
│ Config:          │
│ - condition      │  "{{input.amount}} > 100"
│ - action         │  "continue" / "stop" / "error"
└──────────────────┘
```

---

## 3. HOW USERS BUILD WORKFLOWS (JSON Example)

```json
{
  "workflow": {
    "name": "Payment Processing Pipeline",
    "nodes": [
      {
        "id": "webhook_trigger",
        "type": "WEBHOOK",
        "config": {
          "path": "/webhooks/stripe-payment"
        }
      },
      {
        "id": "call_payment_api",
        "type": "HTTP",
        "config": {
          "url": "https://api.stripe.com/v1/charges",
          "method": "POST",
          "headers": {
            "Authorization": "Bearer {{secrets.stripe_key}}"
          },
          "body": {
            "amount": "{{trigger.amount}}",
            "currency": "usd",
            "source": "{{trigger.token}}"
          }
        }
      },
      {
        "id": "check_payment_status",
        "type": "CONDITIONAL",
        "config": {
          "condition": "{{call_payment_api.output.status}} == 'succeeded'"
        }
      },
      {
        "id": "send_success_email",
        "type": "EMAIL",
        "config": {
          "provider": "sendgrid",
          "to": "{{trigger.customer_email}}",
          "subject": "Payment Successful",
          "body": "Your payment of ${{trigger.amount}} was processed successfully!"
        }
      },
      {
        "id": "send_failure_email",
        "type": "EMAIL",
        "config": {
          "provider": "sendgrid",
          "to": "{{trigger.customer_email}}",
          "subject": "Payment Failed",
          "body": "Unfortunately, your payment could not be processed."
        }
      },
      {
        "id": "notify_slack",
        "type": "SLACK",
        "config": {
          "channel": "#payments",
          "message": "Payment: ${{trigger.amount}} - Status: {{call_payment_api.output.status}}"
        }
      }
    ],
    "edges": [
      {"source": "webhook_trigger", "target": "call_payment_api"},
      {"source": "call_payment_api", "target": "check_payment_status"},
      {"source": "check_payment_status", "target": "send_success_email", "condition": "true"},
      {"source": "check_payment_status", "target": "send_failure_email", "condition": "false"},
      {"source": "send_success_email", "target": "notify_slack"},
      {"source": "send_failure_email", "target": "notify_slack"}
    ]
  }
}
```

### Visual Representation:

```
┌─────────────────┐
│ Webhook Trigger │  (Stripe sends payment data)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ HTTP: Call      │  (Charge credit card via Stripe API)
│ Stripe API      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Conditional:    │  (Check if payment succeeded)
│ Payment Status? │
└────┬───────┬────┘
     │       │
  SUCCESS  FAILED
     │       │
     ▼       ▼
┌─────────┐ ┌─────────┐
│ Email:  │ │ Email:  │
│ Success │ │ Failure │
└────┬────┘ └────┬────┘
     │           │
     └─────┬─────┘
           ▼
     ┌─────────┐
     │ Slack:  │
     │ Notify  │
     └─────────┘
```

---

## 4. INSIDE THE NODE WORKER POOL (How Each Node Type is Executed)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    NODE WORKER POOL SERVICE                              │
│                       (Stateless Executors)                              │
└─────────────────────────────────────────────────────────────────────────┘

                    ┌──────────────────────┐
                    │   RabbitMQ Queue     │
                    │ "orchestration-to-   │
                    │      worker"         │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   Worker Instance    │
                    │   Pulls task from    │
                    │      queue           │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │  Type Router         │
                    │  (Switch statement)  │
                    └──────────┬───────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
        ▼                      ▼                      ▼
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│ HTTP Handler │      │ Email Handler│      │ Slack Handler│
├──────────────┤      ├──────────────┤      ├──────────────┤
│ Uses:        │      │ Uses:        │      │ Uses:        │
│ - net/http   │      │ - SendGrid   │      │ - Slack SDK  │
│ - HTTP client│      │   SDK        │      │              │
│              │      │ - AWS SES    │      │              │
│ Executes:    │      │              │      │ Executes:    │
│ GET/POST/PUT │      │ Executes:    │      │ Posts message│
│ DELETE       │      │ Sends email  │      │ to channel   │
└──────┬───────┘      └──────┬───────┘      └──────┬───────┘
       │                     │                     │
       └─────────────────────┼─────────────────────┘
                             │
                    ┌────────▼─────────┐
                    │  Result Builder  │
                    │  (Success/Error) │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │  Publish Result  │
                    │  to RabbitMQ     │
                    │ "worker-to-      │
                    │ orchestration"   │
                    └──────────────────┘
```

### Code Structure in Worker Pool:

```
node-worker-pool/
├── cmd/
│   └── worker/
│       └── main.go                  ← Entry point
│
├── internal/
│   ├── executor/
│   │   ├── executor.go              ← Main dispatcher
│   │   ├── http_handler.go          ← HTTP node execution
│   │   ├── email_handler.go         ← Email node execution
│   │   ├── slack_handler.go         ← Slack node execution
│   │   ├── transform_handler.go     ← Transform node execution
│   │   ├── delay_handler.go         ← Delay node execution
│   │   ├── database_handler.go      ← Database node execution
│   │   └── filter_handler.go        ← Filter node execution
│   │
│   └── queue/
│       ├── consumer.go              ← Pulls tasks from RabbitMQ
│       └── publisher.go             ← Publishes results back
│
└── go.mod
```

---

## 5. MESSAGE FLOW: Complete Execution Journey

```
USER ACTION: Sends webhook
        │
        ▼
┌───────────────────────────────────────────────────────────────────┐
│ STEP 1: TRIGGER/API SERVICE                                       │
├───────────────────────────────────────────────────────────────────┤
│ 1. Receive webhook POST /webhooks/abc123                          │
│ 2. Look up workflow_id from webhook path                          │
│ 3. Create execution record (status: PENDING)                      │
│ 4. Publish to RabbitMQ: "trigger-to-orchestration"               │
│                                                                    │
│    Message: NewRunMessage {                                        │
│      execution_id: "exec-123"                                      │
│      workflow_id: "wf-456"                                         │
│      trigger_data: {webhook payload}                               │
│      dag_definition: {nodes, edges}                                │
│    }                                                               │
└────────────────────────────┬──────────────────────────────────────┘
                             │
                             ▼
┌───────────────────────────────────────────────────────────────────┐
│ STEP 2: ORCHESTRATION ENGINE                                      │
├───────────────────────────────────────────────────────────────────┤
│ 1. Consume NewRunMessage from queue                               │
│ 2. Parse DAG, find root node (webhook_trigger)                    │
│ 3. Find next executable node: "call_payment_api" (HTTP)           │
│ 4. Interpolate config: replace {{trigger.amount}} with 1000       │
│ 5. Save state to orchestration_db                                 │
│ 6. Publish to RabbitMQ: "orchestration-to-worker"                │
│                                                                    │
│    Message: NodeTaskMessage {                                      │
│      execution_id: "exec-123"                                      │
│      node_id: "call_payment_api"                                   │
│      node_type: "HTTP"                                             │
│      config: {url, method, headers, body} ← fully resolved        │
│    }                                                               │
└────────────────────────────┬──────────────────────────────────────┘
                             │
                             ▼
┌───────────────────────────────────────────────────────────────────┐
│ STEP 3: NODE WORKER POOL                                          │
├───────────────────────────────────────────────────────────────────┤
│ 1. Consume NodeTaskMessage from queue                             │
│ 2. Route to HTTP handler (based on node_type)                     │
│ 3. Execute: POST https://api.stripe.com/v1/charges                │
│ 4. Wait for response (30 seconds timeout)                         │
│ 5. Parse response: {status: "succeeded", charge_id: "ch_123"}     │
│ 6. Publish to RabbitMQ: "worker-to-orchestration"                │
│                                                                    │
│    Message: NodeResultMessage {                                    │
│      execution_id: "exec-123"                                      │
│      node_id: "call_payment_api"                                   │
│      status: "SUCCESS"                                             │
│      output_data: {stripe response}                                │
│    }                                                               │
└────────────────────────────┬──────────────────────────────────────┘
                             │
                             ▼
┌───────────────────────────────────────────────────────────────────┐
│ STEP 4: ORCHESTRATION ENGINE (Again)                              │
├───────────────────────────────────────────────────────────────────┤
│ 1. Consume NodeResultMessage                                       │
│ 2. Update node_executions table: mark "call_payment_api" = SUCCESS│
│ 3. Find next node in DAG: "check_payment_status" (CONDITIONAL)    │
│ 4. Evaluate condition: {{call_payment_api.output.status}} == ...  │
│ 5. Condition = TRUE → follow "true" edge to "send_success_email"  │
│ 6. Publish NodeTaskMessage for "send_success_email" (EMAIL type)  │
└────────────────────────────┬──────────────────────────────────────┘
                             │
                             ▼
        [Repeat STEP 3 & 4 for each remaining node]
                             │
                             ▼
┌───────────────────────────────────────────────────────────────────┐
│ STEP 5: WORKFLOW COMPLETE                                         │
├───────────────────────────────────────────────────────────────────┤
│ 1. Orchestration Engine detects no more nodes to execute          │
│ 2. Marks workflow_runs.status = "COMPLETED"                       │
│ 3. Publishes to RabbitMQ: "orchestration-to-trigger-status"      │
│                                                                    │
│    Message: ExecutionStatusMessage {                               │
│      execution_id: "exec-123"                                      │
│      status: "COMPLETED"                                           │
│      completed_at: "2026-07-19T10:30:00Z"                          │
│    }                                                               │
└────────────────────────────┬──────────────────────────────────────┘
                             │
                             ▼
┌───────────────────────────────────────────────────────────────────┐
│ STEP 6: TRIGGER/API SERVICE (Status Update)                       │
├───────────────────────────────────────────────────────────────────┤
│ 1. Consume ExecutionStatusMessage                                  │
│ 2. Update executions table: status = "COMPLETED"                   │
│ 3. User can now query: GET /v1/executions/exec-123                │
│    Response: {status: "COMPLETED", started_at, completed_at}       │
└───────────────────────────────────────────────────────────────────┘
```

---

## 6. SCALING: How Many Instances of Each Service?

```
PRODUCTION DEPLOYMENT EXAMPLE (AWS ECS)

┌─────────────────────────────────────────────────────────────────┐
│  LOAD BALANCER (ALB)                                            │
│  Public endpoint: https://loom.yourcompany.com                  │
└─────────────────┬───────────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
        ▼                   ▼
┌──────────────┐    ┌──────────────┐
│ Trigger/API  │    │ Trigger/API  │    2-5 instances
│  Instance 1  │    │  Instance 2  │    (handles user requests)
└──────────────┘    └──────────────┘

┌──────────────────────────────────────┐
│ Orchestration Engine                 │    1-3 instances
│  Instance 1                          │    (DAG brain, less load)
└──────────────────────────────────────┘

┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ Worker 1 │ │ Worker 2 │ │ Worker 3 │ │ Worker 4 │ │ Worker 5 │
└──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘
     ↕             ↕             ↕             ↕             ↕
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ Worker 6 │ │ Worker 7 │ │ Worker 8 │ │ Worker 9 │ │ Worker 10│
└──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘

     10-100+ Worker instances (scales based on load)
     Each pulls from the same RabbitMQ queue
```

### Scaling Rules:

| Service | Min Instances | Max Instances | Scale Based On |
|---------|--------------|---------------|----------------|
| **Trigger/API** | 2 | 10 | HTTP request rate, webhook volume |
| **Orchestration Engine** | 1 | 5 | Concurrent workflow count, queue depth |
| **Node Worker Pool** | 5 | 100+ | Task queue depth, execution latency |

---

## 7. SUMMARY: TOTAL COMPONENT COUNT

```
┌─────────────────────────────────────────────────────────────────┐
│                    LOOM COMPLETE INVENTORY                       │
└─────────────────────────────────────────────────────────────────┘

SERVICES (Deployable Components)
├── 1. Trigger/API Service          (Go service, port 8080)
├── 2. Orchestration Engine         (Go service, internal)
└── 3. Node Worker Pool             (Go service, scalable)

INFRASTRUCTURE (Shared Dependencies)
├── 4. PostgreSQL                   (2 databases: trigger_db, orchestration_db)
├── 5. RabbitMQ                     (3 queues + 3 dead-letter queues)
└── 6. Redis                        (Optional: cron deduplication)

NODE TYPES (Building Blocks Users Configure)
├── 7.  HTTP Node
├── 8.  Conditional Node
├── 9.  Transform Node
├── 10. Delay Node
├── 11. Email Node
├── 12. Webhook Trigger
├── 13. Schedule Trigger
├── 14. Database Node              (Future)
├── 15. Slack Node                 (Future)
└── 16. Filter Node                (Future)

TOTAL: 3 Services + 3 Infrastructure + 10 Node Types
```

---

## 8. WHAT EXISTS vs WHAT'S MISSING

```
✅ EXISTS (Built)
├── Trigger/API Service ✅
├── Orchestration Engine ✅
├── PostgreSQL setup ✅
├── RabbitMQ setup ✅
├── Message contracts ✅
├── Database schemas ✅
└── Docker Compose ✅

❌ MISSING (Need to Build)
├── Node Worker Pool Service ❌ ← CRITICAL
├── HTTP node handler ❌
├── Email node handler ❌
├── Transform node handler ❌
├── Delay node handler ❌
├── Status update consumer ❌
├── Condition evaluator ❌
└── Tests ❌
```

