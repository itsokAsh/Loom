# Testing User Onboarding Workflow - Complete Guide

This guide walks you through testing the complete user onboarding email workflow end-to-end.

## Prerequisites

- SendGrid API key configured in `.env`
- Docker and Docker Compose installed
- curl or Postman for API calls
- A real email address you can check (for testing)

---

## Part 1: Start the System

### Step 1: Start All Services

```powershell
# Navigate to project root
cd c:\Users\ashcr\Desktop\Loom\Loom

# Start all services
docker-compose up -d

# Wait ~30 seconds for services to start and migrations to run
Start-Sleep -Seconds 30

# Check service health
docker-compose ps
```

**Expected Output:**
```
NAME                                 STATUS
workflow_orchestration               Up
workflow_postgres                    Up (healthy)
workflow_rabbitmq                    Up (healthy)
workflow_redis                       Up (healthy)
workflow_trigger                     Up
workflow_worker_pool                 Up
```

### Step 2: Verify Services are Ready

```powershell
# Check trigger-api is responding
curl http://localhost:8080/v1/workflows

# Check logs for errors
docker-compose logs --tail=50 trigger-api
docker-compose logs --tail=50 orchestration-engine
docker-compose logs --tail=50 node-worker-pool
```

---

## Part 2: Create the Workflow

### Step 3: Create User Onboarding Workflow

```powershell
# Create the workflow
$workflowResponse = curl -X POST http://localhost:8080/v1/workflows `
  -H "Content-Type: application/json" `
  -d '{
    "name": "User Onboarding with Welcome Email",
    "dag": {
      "nodes": [
        {
          "id": "send_welcome_email",
          "type": "EMAIL",
          "config": {
            "to": "{{trigger.email}}",
            "subject": "Welcome to Our Platform!",
            "body": "Hi {{trigger.name}},\n\nThanks for signing up. We are excited to have you on board!\n\nBest regards,\nThe Team",
            "from": "noreply@loom.com"
          }
        }
      ],
      "edges": []
    }
  }' | ConvertFrom-Json

# Save the workflow ID
$workflowId = $workflowResponse.id
Write-Host "Created workflow with ID: $workflowId"
```

**Alternative using curl directly:**
```powershell
curl -X POST http://localhost:8080/v1/workflows `
  -H "Content-Type: application/json" `
  -d "@examples/user_onboarding_workflow.json"

# Manually save the "id" from the response
```

### Step 4: Verify Workflow Created

```powershell
# Get the workflow details
curl http://localhost:8080/v1/workflows/$workflowId
```

**Expected Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "User Onboarding with Welcome Email",
  "created_at": "2024-01-15T10:00:00Z",
  ...
}
```

---

## Part 3: Set Up Webhook Trigger

### Step 5: Create Webhook for the Workflow

```powershell
# Create webhook
$webhookResponse = curl -X POST http://localhost:8080/v1/workflows/$workflowId/webhooks `
  | ConvertFrom-Json

# Save webhook path and secret
$webhookPath = $webhookResponse.path
$webhookSecret = $webhookResponse.secret

Write-Host "Webhook Path: $webhookPath"
Write-Host "Webhook Secret: $webhookSecret"
Write-Host ""
Write-Host "Full webhook URL: http://localhost:8080/webhooks/$webhookPath"
```

**Save these values!** You'll need them in the next step.

---

## Part 4: Trigger the Workflow

### Step 6: Send Test User Signup

Now simulate a user signup by calling your webhook:

```powershell
# Replace YOUR_EMAIL with your actual email address
$testEmail = "YOUR_EMAIL@example.com"
$testName = "John Doe"

# Generate idempotency key (prevents duplicate sends if you retry)
$idempotencyKey = "test-$(Get-Date -Format 'yyyyMMddHHmmss')"

# Prepare payload
$payload = @{
    email = $testEmail
    name = $testName
    signup_date = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
} | ConvertTo-Json

# For testing, we'll skip HMAC (in production you MUST include it)
# Send the webhook request
$executionResponse = curl -X POST "http://localhost:8080/webhooks/$webhookPath" `
  -H "Content-Type: application/json" `
  -H "X-Idempotency-Key: $idempotencyKey" `
  -d $payload | ConvertFrom-Json

# Save execution ID
$executionId = $executionResponse.execution_id
Write-Host "Workflow triggered! Execution ID: $executionId"
```

**With Proper HMAC (Production):**
```powershell
# Calculate HMAC signature
$hmac = New-Object System.Security.Cryptography.HMACSHA256
$hmac.Key = [Text.Encoding]::UTF8.GetBytes($webhookSecret)
$signature = [Convert]::ToHexString($hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($payload))).ToLower()

# Send with signature
curl -X POST "http://localhost:8080/webhooks/$webhookPath" `
  -H "Content-Type: application/json" `
  -H "X-Signature: sha256=$signature" `
  -H "X-Idempotency-Key: $idempotencyKey" `
  -d $payload
```

---

## Part 5: Monitor Execution

### Step 7: Check Execution Status

```powershell
# Wait a few seconds for processing
Start-Sleep -Seconds 5

# Check execution status
curl http://localhost:8080/v1/executions/$executionId | ConvertFrom-Json | Format-List

# Keep checking until status is COMPLETED or FAILED
while ($true) {
    $status = (curl http://localhost:8080/v1/executions/$executionId | ConvertFrom-Json).status
    Write-Host "Status: $status"
    if ($status -eq "COMPLETED" -or $status -eq "FAILED") {
        break
    }
    Start-Sleep -Seconds 2
}
```

**Expected Status Progression:**
1. `PENDING` - Workflow created, waiting for orchestration engine
2. `RUNNING` - Orchestration engine processing
3. `COMPLETED` - Email sent successfully

**If Status is FAILED:**
```powershell
# Check execution details for error
curl http://localhost:8080/v1/executions/$executionId

# Check service logs
docker-compose logs --tail=100 orchestration-engine
docker-compose logs --tail=100 node-worker-pool
```

### Step 8: Check Service Logs

```powershell
# Check orchestration engine logs (should show workflow processing)
docker-compose logs --tail=50 orchestration-engine | Select-String -Pattern $executionId

# Check worker logs (should show email being sent)
docker-compose logs --tail=50 node-worker-pool | Select-String -Pattern $executionId

# Look for specific messages
docker-compose logs node-worker-pool | Select-String -Pattern "EMAIL"
docker-compose logs node-worker-pool | Select-String -Pattern "SendGrid"
```

**What to Look For:**
- Orchestration engine: `Email dispatch count for execution`
- Worker: `Email sent successfully` or `SendGrid API`
- No ERROR messages

---

## Part 6: Verify Email Delivery

### Step 9: Check SendGrid Dashboard

1. Go to https://app.sendgrid.com/email_activity
2. Look for your email in the activity feed (may take 1-2 minutes)
3. Check the status:
   - **Processed** - SendGrid accepted it
   - **Delivered** - Successfully delivered to inbox
   - **Bounce** - Invalid email or delivery failed

### Step 10: Check Your Inbox

1. Open the email inbox for the address you used
2. Look for email with subject: "Welcome to Our Platform!"
3. Check spam/junk folder if not in inbox
4. Verify the email content matches your template

---

## Part 7: Advanced Testing

### Test Multiple Users (Batch)

```powershell
# Send 5 test signups
$users = @(
    @{email="user1@test.com"; name="Alice"},
    @{email="user2@test.com"; name="Bob"},
    @{email="user3@test.com"; name="Charlie"},
    @{email="user4@test.com"; name="Diana"},
    @{email="user5@test.com"; name="Eve"}
)

foreach ($user in $users) {
    $payload = $user | ConvertTo-Json
    $idempotencyKey = "batch-$($user.name)-$(Get-Date -Format 'yyyyMMddHHmmss')"
    
    curl -X POST "http://localhost:8080/webhooks/$webhookPath" `
      -H "Content-Type: application/json" `
      -H "X-Idempotency-Key: $idempotencyKey" `
      -d $payload
    
    Write-Host "Sent signup for $($user.name)"
    Start-Sleep -Seconds 1
}
```

### Test Email Counter Limit

```powershell
# This workflow should skip the 101st email
# Create a workflow with 101 email nodes
$manyEmailNodes = 1..101 | ForEach-Object {
    @{
        id = "email_$_"
        type = "EMAIL"
        config = @{
            to = "test@example.com"
            subject = "Email #$_"
            body = "This is email number $_"
        }
    }
}

$testWorkflow = @{
    name = "Email Limit Test"
    dag = @{
        nodes = $manyEmailNodes
        edges = @()
    }
} | ConvertTo-Json -Depth 10

# Create and trigger this workflow
# The 101st email should be SKIPPED (check logs)
```

### Test with Different Trigger Data

```powershell
# Test with minimal data
curl -X POST "http://localhost:8080/webhooks/$webhookPath" `
  -H "Content-Type: application/json" `
  -H "X-Idempotency-Key: minimal-$(Get-Date -Format 'yyyyMMddHHmmss')" `
  -d '{"email":"test@example.com","name":"Test User"}'

# Test with extra data (should be ignored but available in trigger context)
curl -X POST "http://localhost:8080/webhooks/$webhookPath" `
  -H "Content-Type: application/json" `
  -H "X-Idempotency-Key: extra-$(Get-Date -Format 'yyyyMMddHHmmss')" `
  -d '{"email":"test@example.com","name":"Test","plan":"premium","source":"referral"}'
```

---

## Troubleshooting

### Problem: Status stays PENDING

**Cause:** Orchestration engine not consuming messages

**Fix:**
```powershell
# Check if orchestration engine is running
docker-compose ps orchestration-engine

# Check logs for errors
docker-compose logs orchestration-engine

# Restart orchestration engine
docker-compose restart orchestration-engine
```

### Problem: Status is FAILED

**Cause:** Email node failed (invalid config, SendGrid error, etc.)

**Fix:**
```powershell
# Get execution details
curl http://localhost:8080/v1/executions/$executionId

# Check worker logs for error message
docker-compose logs node-worker-pool | Select-String -Pattern "ERROR"

# Common errors:
# - "secret not found: SENDGRID_API_KEY" → Add API key to .env
# - "invalid recipient email" → Check email format
# - "SendGrid API error: status 401" → Check API key
# - "SendGrid API error: status 403" → Check API key permissions
```

### Problem: Email not received

**Checks:**
1. **SendGrid Activity:** https://app.sendgrid.com/email_activity
2. **Spam Folder:** Check junk/spam
3. **From Address:** Verify sender domain in SendGrid
4. **Execution Status:** Should be COMPLETED
5. **Worker Logs:** Should show "sent successfully"

```powershell
# Verify SendGrid API key works
$apiKey = $env:SENDGRID_API_KEY
curl https://api.sendgrid.com/v3/mail/send `
  -H "Authorization: Bearer $apiKey" `
  -H "Content-Type: application/json" `
  -d '{
    "personalizations":[{"to":[{"email":"test@example.com"}]}],
    "from":{"email":"noreply@loom.com"},
    "subject":"Test",
    "content":[{"type":"text/plain","value":"Test"}]
  }'
```

### Problem: Worker not picking up tasks

**Cause:** RabbitMQ connection issue or worker crashed

**Fix:**
```powershell
# Check RabbitMQ queues
# Open: http://localhost:15672 (guest/guest)
# Look at "orchestration-to-worker" queue
# Should have consumers = 1, messages should drain

# Check worker status
docker-compose logs node-worker-pool --tail=100

# Restart worker
docker-compose restart node-worker-pool
```

---

## Complete Test Script (Copy-Paste)

Here's a complete PowerShell script you can save and run:

```powershell
# Save this as: test-user-onboarding.ps1

Write-Host "=== User Onboarding Workflow Test ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Start services
Write-Host "Step 1: Starting services..." -ForegroundColor Yellow
docker-compose up -d
Start-Sleep -Seconds 30

# Step 2: Create workflow
Write-Host "Step 2: Creating workflow..." -ForegroundColor Yellow
$workflowJson = Get-Content examples/user_onboarding_workflow.json -Raw
$workflow = curl -X POST http://localhost:8080/v1/workflows `
  -H "Content-Type: application/json" `
  -d $workflowJson | ConvertFrom-Json
$workflowId = $workflow.id
Write-Host "  Created: $workflowId" -ForegroundColor Green

# Step 3: Create webhook
Write-Host "Step 3: Creating webhook..." -ForegroundColor Yellow
$webhook = curl -X POST "http://localhost:8080/v1/workflows/$workflowId/webhooks" `
  | ConvertFrom-Json
$webhookPath = $webhook.path
Write-Host "  Path: $webhookPath" -ForegroundColor Green

# Step 4: Trigger workflow
Write-Host "Step 4: Triggering workflow..." -ForegroundColor Yellow
$testEmail = Read-Host "Enter your email address for testing"
$payload = @{email=$testEmail; name="Test User"} | ConvertTo-Json
$execution = curl -X POST "http://localhost:8080/webhooks/$webhookPath" `
  -H "Content-Type: application/json" `
  -H "X-Idempotency-Key: test-$(Get-Date -Format 'yyyyMMddHHmmss')" `
  -d $payload | ConvertFrom-Json
$executionId = $execution.execution_id
Write-Host "  Execution: $executionId" -ForegroundColor Green

# Step 5: Wait for completion
Write-Host "Step 5: Waiting for completion..." -ForegroundColor Yellow
Start-Sleep -Seconds 5
$maxWait = 30
$waited = 0
while ($waited -lt $maxWait) {
    $status = (curl "http://localhost:8080/v1/executions/$executionId" | ConvertFrom-Json).status
    Write-Host "  Status: $status" -ForegroundColor Cyan
    if ($status -eq "COMPLETED") {
        Write-Host "  SUCCESS! Email sent." -ForegroundColor Green
        break
    }
    if ($status -eq "FAILED") {
        Write-Host "  FAILED! Check logs." -ForegroundColor Red
        docker-compose logs --tail=50 node-worker-pool
        break
    }
    Start-Sleep -Seconds 2
    $waited += 2
}

Write-Host ""
Write-Host "=== Test Complete ===" -ForegroundColor Cyan
Write-Host "Check your inbox: $testEmail" -ForegroundColor Yellow
Write-Host "SendGrid Activity: https://app.sendgrid.com/email_activity" -ForegroundColor Yellow
```

**Run it:**
```powershell
.\test-user-onboarding.ps1
```

---

## Success Checklist

- [x] Services started successfully
- [x] Workflow created (got workflow ID)
- [x] Webhook created (got webhook path)
- [x] Workflow triggered (got execution ID)
- [x] Execution status → COMPLETED
- [x] Email visible in SendGrid Activity
- [x] Email received in inbox

**If all checked: 🎉 Your user onboarding workflow is working!**
