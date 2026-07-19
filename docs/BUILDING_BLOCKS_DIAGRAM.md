# Loom Building Blocks - Simplified View

## THE ANSWER TO YOUR QUESTIONS

### Question 1: "How many services?"
**Answer: 3 services + 3 infrastructure components**

### Question 2: "What are the predefined building blocks?"
**Answer: 10 node types (like LEGO pieces)**

---

## SIMPLIFIED ARCHITECTURE

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              YOUR LOOM SYSTEM                                    │
└─────────────────────────────────────────────────────────────────────────────────┘

                              ┌─────────────────┐
                              │   THE 3 CORE    │
                              │    SERVICES     │
                              └─────────────────┘
                                      │
                ┌─────────────────────┼─────────────────────┐
                │                     │                     │
                ▼                     ▼                     ▼
        ┌───────────────┐     ┌───────────────┐    ┌──────────────┐
        │   SERVICE 1   │     │   SERVICE 2   │    │  SERVICE 3   │
        │               │     │               │    │              │
        │  TRIGGER/API  │────▶│ ORCHESTRATION │───▶│   WORKERS    │
        │               │     │    ENGINE     │    │              │
        │ "Front Door"  │     │   "The Brain" │    │  "The Hands" │
        └───────────────┘     └───────────────┘    └──────────────┘
              │                      │                     │
              │                      │                     │
              ▼                      ▼                     ▼
        Workflows API         DAG Traversal         Execute Tasks
        Webhooks              Conditions            HTTP calls
        Schedules             State Machine         Emails
        History                                     Transforms


                        ┌────────────────────────┐
                        │ INFRASTRUCTURE (Shared)│
                        ├────────────────────────┤
                        │ PostgreSQL x2          │
                        │ RabbitMQ               │
                        │ Redis                  │
                        └────────────────────────┘
```

---

## THE 10 BUILDING BLOCKS (Node Types)

Users compose workflows using these predefined types:

```
╔══════════════════════════════════════════════════════════════════╗
║                  NODE TYPE CATALOG                                ║
║              (What Users Can Configure)                           ║
╚══════════════════════════════════════════════════════════════════╝

┌─────────────┐
│ 1. WEBHOOK  │  ← Entry point: receives external HTTP POST
└─────────────┘
       │
       ▼
┌─────────────┐
│ 2. HTTP     │  ← Calls any REST API (GET/POST/PUT/DELETE)
└─────────────┘
       │
       ▼
┌─────────────┐
│ 3. CONDITION│  ← IF/ELSE branching (like: if payment > $100)
└─────────────┘
     │   │
  YES│   │NO
     │   │
     ▼   ▼
┌──────┐ ┌──────┐
│ PATH │ │ PATH │
│  A   │ │  B   │
└──────┘ └──────┘
     │     │
     └──┬──┘
        ▼
┌─────────────┐
│ 4. TRANSFORM│  ← Data manipulation (map/filter/modify JSON)
└─────────────┘
       │
       ▼
┌─────────────┐
│ 5. DELAY    │  ← Wait X seconds/minutes/hours
└─────────────┘
       │
       ▼
┌─────────────┐
│ 6. EMAIL    │  ← Send email via SendGrid/AWS SES/SMTP
└─────────────┘
       │
       ▼
┌─────────────┐
│ 7. SLACK    │  ← Post message to Slack channel
└─────────────┘
       │
       ▼
┌─────────────┐
│ 8. DATABASE │  ← Query/Insert into PostgreSQL/MySQL
└─────────────┘
       │
       ▼
┌─────────────┐
│ 9. FILTER   │  ← Stop workflow if condition not met
└─────────────┘
       │
       ▼
┌─────────────┐
│10. SCHEDULE │  ← Cron trigger (runs at specific times)
└─────────────┘

Each block is PRECODED in the Worker Pool.
Users just CONFIGURE them via JSON.
```

---

## HOW IT WORKS: A REAL EXAMPLE

### User creates this workflow:

```json
{
  "nodes": [
    {"id": "trigger", "type": "WEBHOOK"},
    {"id": "charge", "type": "HTTP", "config": {"url": "stripe.com"}},
    {"id": "check", "type": "CONDITION", "config": {"condition": "amount > 100"}},
    {"id": "email1", "type": "EMAIL", "config": {"to": "user@x.com", "subject": "Success"}},
    {"id": "email2", "type": "EMAIL", "config": {"to": "user@x.com", "subject": "Failed"}},
    {"id": "notify", "type": "SLACK", "config": {"channel": "#sales"}}
  ],
  "edges": [
    {"source": "trigger", "target": "charge"},
    {"source": "charge", "target": "check"},
    {"source": "check", "target": "email1", "condition": "true"},
    {"source": "check", "target": "email2", "condition": "false"},
    {"source": "email1", "target": "notify"},
    {"source": "email2", "target": "notify"}
  ]
}
```

### Visual representation:

```
                    ┌──────────┐
                    │ WEBHOOK  │  ← Stripe sends payment webhook
                    └─────┬────┘
                          │
                          ▼
                    ┌──────────┐
                    │   HTTP   │  ← Call Stripe API to charge card
                    └─────┬────┘
                          │
                          ▼
                  ┌───────────────┐
                  │  CONDITIONAL  │  ← Check if amount > $100
                  └───┬───────┬───┘
                      │       │
                   YES│       │NO
                      │       │
                      ▼       ▼
                ┌─────────┐ ┌─────────┐
                │ EMAIL   │ │ EMAIL   │
                │ Success │ │ Failed  │
                └────┬────┘ └────┬────┘
                     │           │
                     └─────┬─────┘
                           ▼
                     ┌─────────┐
                     │  SLACK  │  ← Notify team
                     └─────────┘
```

### What happens behind the scenes:

```
1. USER: curl -X POST /webhooks/abc123 -d '{"amount": 150}'

2. SERVICE 1 (Trigger/API):
   ✓ Receives webhook
   ✓ Creates execution record
   ✓ Sends message to RabbitMQ: "Start workflow X"

3. SERVICE 2 (Orchestration Engine):
   ✓ Reads workflow JSON
   ✓ Sees first node is "HTTP" type
   ✓ Sends message to RabbitMQ: "Execute HTTP task"

4. SERVICE 3 (Worker Pool):
   ✓ Receives "Execute HTTP task"
   ✓ Looks up handler for "HTTP" type
   ✓ Executes: POST to stripe.com
   ✓ Gets result: {"status": "success", "charge_id": "ch_123"}
   ✓ Sends message back: "Task completed successfully"

5. SERVICE 2 (Orchestration Engine):
   ✓ Receives "Task completed"
   ✓ Looks at next node: "CONDITION"
   ✓ Evaluates: amount (150) > 100? YES
   ✓ Follows "true" path to "email1"
   ✓ Sends message: "Execute EMAIL task"

6. SERVICE 3 (Worker Pool):
   ✓ Receives "Execute EMAIL task"
   ✓ Looks up handler for "EMAIL" type
   ✓ Executes: Send email via SendGrid
   ✓ Sends message back: "Task completed"

7. [Continues until all nodes executed]

8. SERVICE 2 (Orchestration Engine):
   ✓ No more nodes
   ✓ Marks workflow as COMPLETED
   ✓ Sends status update to SERVICE 1

9. SERVICE 1 (Trigger/API):
   ✓ Updates execution status to COMPLETED
   ✓ User can query: GET /v1/executions/123
```

---

## WHERE THE CODE LIVES

```
loom/
│
├── trigger-api/                    ← SERVICE 1 ✅ EXISTS
│   ├── cmd/api/main.go
│   └── internal/
│       ├── workflows/             (CRUD operations)
│       ├── webhooks/              (Receive webhooks)
│       └── schedules/             (Cron jobs)
│
├── orchestration-engine/           ← SERVICE 2 ✅ EXISTS
│   ├── cmd/engine/main.go
│   └── internal/
│       ├── dag/                   (DAG traversal)
│       └── engine/                (State machine)
│
├── node-worker-pool/               ← SERVICE 3 ❌ MISSING
│   ├── cmd/worker/main.go
│   └── internal/
│       └── handlers/
│           ├── http_handler.go    ← Handler for HTTP nodes
│           ├── email_handler.go   ← Handler for EMAIL nodes
│           ├── slack_handler.go   ← Handler for SLACK nodes
│           ├── transform_handler.go
│           ├── delay_handler.go
│           ├── database_handler.go
│           └── filter_handler.go
│
└── shared/
    └── queue-contracts/            ← Message definitions
        └── events.go
```

---

## KEY INSIGHT: NO AI NEEDED

```
❌ WRONG ASSUMPTION:
"User sends arbitrary code → System needs AI to understand it"

✅ CORRECT MODEL:
"User configures predefined blocks → System executes known handlers"

It's like:
┌─────────────────────────────────────────────────────────┐
│ LEGO Analogy                                            │
├─────────────────────────────────────────────────────────┤
│ User doesn't create new LEGO shapes                     │
│ User picks from existing pieces (2x4 brick, wheel, etc)│
│ User arranges them in different combinations            │
│                                                          │
│ Loom = Same concept                                     │
│ - 10 predefined node types (the LEGO pieces)           │
│ - User arranges them in JSON (the instructions)        │
│ - Worker Pool executes each type (knows how to handle) │
└─────────────────────────────────────────────────────────┘
```

---

## ADDING NEW NODE TYPES (How to Extend)

### Example: Add "SMS" node type

**Step 1: Define in shared contracts**
```go
// shared/queue-contracts/events.go
const NodeTypeSMS = "SMS"
```

**Step 2: Add handler in worker pool**
```go
// node-worker-pool/internal/handlers/sms_handler.go
func HandleSMS(task NodeTaskMessage) NodeResultMessage {
    // Parse config
    var config struct {
        To      string `json:"to"`
        Message string `json:"message"`
        Provider string `json:"provider"` // "twilio", "sns"
    }
    json.Unmarshal(task.Config, &config)
    
    // Execute
    client := twilio.NewClient(apiKey)
    err := client.SendSMS(config.To, config.Message)
    
    // Return result
    if err != nil {
        return NodeResultMessage{Status: "ERROR", ErrorMessage: err.Error()}
    }
    return NodeResultMessage{Status: "SUCCESS"}
}
```

**Step 3: Register in dispatcher**
```go
// node-worker-pool/internal/executor/executor.go
switch task.NodeType {
    case "HTTP":
        return handleHTTP(task)
    case "EMAIL":
        return handleEmail(task)
    case "SMS":              // ← NEW
        return handleSMS(task) // ← NEW
}
```

**Step 4: Users can now use it**
```json
{
  "id": "send_sms",
  "type": "SMS",
  "config": {
    "provider": "twilio",
    "to": "+1234567890",
    "message": "Your code is 1234"
  }
}
```

---

## FINAL SUMMARY

### Your Questions Answered:

**Q1: "How many services?"**
→ **3 services**: Trigger/API, Orchestration Engine, Worker Pool

**Q2: "How many building blocks?"**
→ **10 node types** (can extend to 20+)

**Q3: "How does system understand user's workflow?"**
→ User picks from **predefined types**, system has **handlers for each**

**Q4: "How to handle different tasks?"**
→ Each worker has **built-in code** for each node type

**Q5: "No AI needed?"**
→ **Correct!** It's configuration, not code generation

