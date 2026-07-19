# Troubleshooting Guide

## Common Issues and Solutions

### Migration Error: "service trigger-migrations didn't complete successfully: exit 1"

**Symptom:**
```
service "trigger-migrations" didn't complete successfully: exit 1
error: migration failed: relation "workflows" already exists
```

**Cause:** The database tables already exist from a previous run. The migrate tool tries to run migrations even when they're already applied.

**Solution:**

**Option 1: Manual start (Quick)**
```powershell
# Start trigger-api manually
docker start workflow_trigger

# Verify it's running
docker-compose ps
```

**Option 2: Clean restart (Recommended for development)**
```powershell
# Stop all services
docker-compose down

# Remove volumes (WARNING: This deletes all data!)
docker-compose down -v

# Start fresh
docker-compose up -d
```

**Option 3: Fix migration state**
```powershell
# Connect to database
docker exec -it workflow_postgres psql -U postgres -d trigger_db

# Check migration version
SELECT * FROM schema_migrations;

# If needed, manually set correct version (matches your migration filename)
# For trigger_db: version should be 1 (from 000001_init_schema.up.sql)
INSERT INTO schema_migrations (version, dirty) VALUES (1, false) 
ON CONFLICT (version) DO UPDATE SET dirty = false;

# Exit
\q

# Restart services
docker-compose restart
```

---

### Email Not Sent

**Symptom:** Workflow completes but no email received

**Checks:**

1. **Verify SendGrid API key is set:**
```powershell
# Check .env file
cat .env | Select-String "SENDGRID_API_KEY"

# Restart worker if you just added the key
docker-compose restart node-worker-pool
```

2. **Check execution status:**
```powershell
# Replace {execution_id} with your actual execution ID
curl http://localhost:8080/v1/executions/{execution_id}
```

3. **Check worker logs:**
```powershell
# Look for errors
docker-compose logs node-worker-pool | Select-String -Pattern "ERROR"

# Look for email activity
docker-compose logs node-worker-pool | Select-String -Pattern "EMAIL"
```

4. **Check SendGrid Activity:**
- Go to https://app.sendgrid.com/email_activity
- Look for your email
- Check status (Processed, Delivered, Bounce, etc.)

**Common Errors:**

| Error Message | Solution |
|---------------|----------|
| `secret not found: SENDGRID_API_KEY` | Add API key to .env and restart worker |
| `SendGrid API error: status 401` | API key is invalid, generate new one |
| `SendGrid API error: status 403` | API key lacks "Mail Send" permission |
| `invalid recipient email` | Check email format in workflow config |
| `blocked destination: api.sendgrid.com resolves to blocked IP` | DNS issue - should not happen with SendGrid |

---

### Workflow Status Stuck on PENDING

**Symptom:** Execution status stays "PENDING" and never progresses

**Cause:** Orchestration engine not consuming messages from RabbitMQ

**Solution:**

1. **Check orchestration engine is running:**
```powershell
docker-compose ps orchestration-engine
docker-compose logs orchestration-engine --tail=50
```

2. **Check RabbitMQ queue:**
- Open http://localhost:15672 (username: guest, password: guest)
- Go to "Queues" tab
- Check `trigger-to-orchestration` queue
- Should have messages = 0 (consumed) and consumers = 1

3. **Restart orchestration engine:**
```powershell
docker-compose restart orchestration-engine

# Wait a few seconds and check again
curl http://localhost:8080/v1/executions/{execution_id}
```

---

### Workflow Status is FAILED

**Symptom:** Execution status shows "FAILED"

**Diagnosis:**

```powershell
# Get execution details
curl http://localhost:8080/v1/executions/{execution_id}

# Check orchestration logs
docker-compose logs orchestration-engine | Select-String -Pattern {execution_id}

# Check worker logs
docker-compose logs node-worker-pool | Select-String -Pattern {execution_id}
```

**Common Causes:**

1. **Email validation failed**
   - Check email format
   - Check subject/body length
   - Check recipient count

2. **SendGrid API error**
   - Check API key is valid
   - Check API key has permissions
   - Check SendGrid account status

3. **Node type not found**
   - Verify node type in workflow JSON
   - Check worker logs for "no executor found"

---

### Port Already in Use

**Symptom:**
```
Error: bind: address already in use
```

**Solution:**

```powershell
# Check what's using the port (replace 8080 with your port)
netstat -ano | findstr :8080

# Kill the process (replace PID with actual process ID)
taskkill /PID <PID> /F

# Or change the port in docker-compose.yml
# From: "8080:8080"
# To:   "8081:8080"
```

---

### Database Connection Error

**Symptom:**
```
failed to connect to database
connection refused
```

**Solution:**

```powershell
# Check postgres is running and healthy
docker-compose ps postgres

# Check postgres logs
docker-compose logs postgres --tail=50

# Restart postgres
docker-compose restart postgres

# Wait for health check to pass
Start-Sleep -Seconds 10

# Restart dependent services
docker-compose restart trigger-api orchestration-engine
```

---

### RabbitMQ Connection Error

**Symptom:**
```
failed to connect to RabbitMQ
Exception (504) Reason: "channel/connection is not open"
```

**Solution:**

```powershell
# Check RabbitMQ is running and healthy
docker-compose ps rabbitmq

# Check RabbitMQ logs
docker-compose logs rabbitmq --tail=50

# Restart RabbitMQ
docker-compose restart rabbitmq

# Wait for health check
Start-Sleep -Seconds 10

# Restart services that connect to RabbitMQ
docker-compose restart trigger-api orchestration-engine node-worker-pool
```

---

### Docker Compose Commands Reference

```powershell
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Stop and remove volumes (deletes data!)
docker-compose down -v

# View logs for all services
docker-compose logs

# View logs for specific service
docker-compose logs trigger-api
docker-compose logs orchestration-engine
docker-compose logs node-worker-pool

# Follow logs in real-time
docker-compose logs -f node-worker-pool

# Restart a service
docker-compose restart trigger-api

# Rebuild and restart a service
docker-compose up -d --build trigger-api

# Check service status
docker-compose ps

# Scale workers
docker-compose up -d --scale node-worker-pool=3
```

---

### Health Check Commands

```powershell
# Check all services
docker-compose ps

# Check trigger-api responds
curl http://localhost:8080/v1/workflows

# Check postgres
docker exec workflow_postgres psql -U postgres -c "SELECT 1"

# Check RabbitMQ
curl http://localhost:15672/api/overview -u guest:guest

# Check redis
docker exec workflow_redis redis-cli ping

# Check database tables exist
docker exec workflow_postgres psql -U postgres -d trigger_db -c "\dt"
docker exec workflow_postgres psql -U postgres -d orchestration_db -c "\dt"

# Check email_dispatch_count column exists
docker exec workflow_postgres psql -U postgres -d orchestration_db -c "\d workflow_runs"
```

---

### Reset Everything (Nuclear Option)

**WARNING: This deletes all data!**

```powershell
# Stop everything
docker-compose down -v

# Remove any orphaned containers
docker-compose rm -f

# Start fresh
docker-compose up -d

# Wait for services to start
Start-Sleep -Seconds 30

# Verify health
docker-compose ps
```

---

### Testing Checklist

After fixing issues, verify everything works:

```powershell
# 1. Services running
docker-compose ps

# 2. API responding
curl http://localhost:8080/v1/workflows

# 3. Database tables exist
docker exec workflow_postgres psql -U postgres -d orchestration_db -c "\d workflow_runs" | Select-String "email_dispatch_count"

# 4. RabbitMQ accessible
curl http://localhost:15672/api/overview -u guest:guest

# 5. Run test workflow
.\test-user-onboarding.ps1 -TestEmail "your@email.com"
```

---

## Getting Help

If you're still stuck:

1. **Collect logs:**
```powershell
# Save all logs to file
docker-compose logs > logs.txt
```

2. **Check service status:**
```powershell
docker-compose ps > status.txt
```

3. **Check database state:**
```powershell
docker exec workflow_postgres psql -U postgres -d orchestration_db -c "SELECT * FROM workflow_runs LIMIT 5" > db-state.txt
```

4. **Share:**
- logs.txt
- status.txt
- db-state.txt
- Your workflow JSON
- The exact error message
