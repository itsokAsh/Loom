# Quick Start: Email Node

This guide will help you send your first email using Loom's Email node in under 10 minutes.

## Prerequisites

- Docker and Docker Compose installed
- SendGrid account (free tier works fine)
- curl or Postman for API calls

## Step 1: Get SendGrid API Key (2 minutes)

1. Go to https://app.sendgrid.com/settings/api_keys
2. Click "Create API Key"
3. Name it "Loom Integration"
4. Select "Full Access" (or at minimum "Mail Send")
5. Click "Create & View"
6. Copy the API key (starts with `SG.`)

## Step 2: Configure Environment (1 minute)

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and paste your SendGrid API key
echo "SENDGRID_API_KEY=SG.your_key_here" > .env
```

## Step 3: Start Services (2 minutes)

```bash
# Start all services
docker-compose up -d

# Wait for services to be healthy (check every few seconds)
docker-compose ps

# You should see all services as "Up" or "healthy"
```

## Step 4: Create Workflow (1 minute)

```bash
# Create a workflow that sends welcome emails
curl -X POST http://localhost:8080/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Welcome Email",
    "dag": {
      "nodes": [
        {
          "id": "send_email",
          "type": "EMAIL",
          "config": {
            "to": "{{trigger.email}}",
            "subject": "Welcome!",
            "body": "Hi {{trigger.name}}, welcome to our platform!"
          }
        }
      ],
      "edges": []
    }
  }'

# Save the workflow_id from the response
```

## Step 5: Create Webhook (1 minute)

```bash
# Replace {workflow_id} with the ID from step 4
curl -X POST http://localhost:8080/v1/workflows/{workflow_id}/webhooks

# Response will include:
# - "path": the webhook URL path
# - "secret": the HMAC secret for signatures

# Save both values!
```

## Step 6: Send Test Email (1 minute)

```bash
# Replace {path} with the webhook path from step 5
# For testing, you can skip HMAC signature (or generate it properly for production)

curl -X POST http://localhost:8080/webhooks/{path} \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: test-$(date +%s)" \
  -d '{
    "email": "your.email@example.com",
    "name": "John Doe"
  }'

# You should get a 202 Accepted response
```

## Step 7: Verify Email Sent (2 minutes)

### Check Execution Status

```bash
# Get execution_id from step 6 response, then:
curl http://localhost:8080/v1/executions/{execution_id}

# Look for "status": "COMPLETED"
```

### Check SendGrid Dashboard

1. Go to https://app.sendgrid.com/email_activity
2. You should see your email in the activity feed
3. Status should be "Delivered" or "Processed"

### Check Your Inbox

- Look for the welcome email
- Check spam/junk folder if not in inbox

---

## 🎉 Success!

You've just sent your first email through Loom's workflow engine!

## Next Steps

### Try Advanced Features

**Add CC/BCC:**
```json
{
  "to": "{{trigger.email}}",
  "cc": ["manager@example.com"],
  "bcc": ["archive@example.com"],
  "subject": "Welcome",
  "body": "Hello!"
}
```

**Multi-Step Workflow:**
```json
{
  "nodes": [
    {
      "id": "fetch_user_data",
      "type": "HTTP",
      "config": {
        "url": "https://api.example.com/users/{{trigger.user_id}}",
        "method": "GET"
      }
    },
    {
      "id": "send_email",
      "type": "EMAIL",
      "config": {
        "to": "{{outputs.fetch_user_data.email}}",
        "subject": "Hi {{outputs.fetch_user_data.name}}!",
        "body": "Your account is ready."
      }
    }
  ],
  "edges": [
    {
      "source": "fetch_user_data",
      "target": "send_email"
    }
  ]
}
```

**Conditional Emails:**
```json
{
  "edges": [
    {
      "source": "check_subscription",
      "target": "send_premium_email",
      "condition": "outputs.check_subscription.tier == 'premium'"
    },
    {
      "source": "check_subscription",
      "target": "send_basic_email",
      "condition": "outputs.check_subscription.tier == 'basic'"
    }
  ]
}
```

### Scale Up

```bash
# Run 5 worker instances for higher throughput
docker-compose up -d --scale node-worker-pool=5
```

### Monitor

```bash
# View worker logs
docker-compose logs -f node-worker-pool

# View orchestration logs
docker-compose logs -f orchestration-engine
```

---

## Troubleshooting

### Email Not Received

1. **Check SendGrid Activity:**
   - Go to https://app.sendgrid.com/email_activity
   - Look for your email
   - Check delivery status and any error messages

2. **Check API Key:**
   ```bash
   # Test API key directly
   curl https://api.sendgrid.com/v3/mail/send \
     -H "Authorization: Bearer $SENDGRID_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "personalizations": [{"to": [{"email": "test@example.com"}]}],
       "from": {"email": "noreply@loom.com"},
       "subject": "Test",
       "content": [{"type": "text/plain", "value": "Test"}]
     }'
   ```

3. **Check Execution Status:**
   ```bash
   # Should be COMPLETED, not FAILED
   curl http://localhost:8080/v1/executions/{execution_id}
   ```

4. **Check Worker Logs:**
   ```bash
   docker-compose logs node-worker-pool | grep ERROR
   ```

### Common Errors

| Error | Solution |
|-------|----------|
| `secret not found: SENDGRID_API_KEY` | Add API key to .env file and restart services |
| `SendGrid API error: status 401` | API key is invalid, generate a new one |
| `SendGrid API error: status 403` | API key lacks "Mail Send" permission |
| `invalid recipient email` | Check email format in workflow config |
| `execution_id not found` | Wait a few seconds, workflow may still be running |

### Need More Help?

- Check detailed logs: `docker-compose logs -f`
- Read the full documentation: `docs/EMAIL_NODE_USAGE.md`
- Review the implementation plan: `docs/SECURE_EMAIL_NODE_IMPLEMENTATION_PLAN.md`
- Check SendGrid documentation: https://docs.sendgrid.com/

---

## Production Checklist

Before going to production:

- [ ] Use proper HMAC signatures for webhooks (don't skip X-Signature header)
- [ ] Verify sender domain in SendGrid (required for deliverability)
- [ ] Set up proper "from" email addresses (not noreply@loom.com)
- [ ] Configure appropriate rate limits
- [ ] Set up monitoring and alerting
- [ ] Test with production email volume
- [ ] Review SendGrid best practices for deliverability

---

**Total Time:** ~10 minutes to send your first email! 🚀
