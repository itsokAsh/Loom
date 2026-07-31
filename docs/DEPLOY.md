# Loom deployment guide

Deploy the full stack (Postgres, RabbitMQ, trigger-api, orchestration-engine, worker pool, React UI) on a single VPS with Docker Compose.

## What gets deployed

| Container | Role | Public in prod? |
|-----------|------|-----------------|
| `ui` | React builder + nginx (`/v1` → API) | Yes (port 3000 or via Caddy) |
| `trigger-api` | Workflows, webhooks, schedules, executions | Internal only |
| `orchestration-engine` | DAG orchestration | Internal only |
| `node-worker-pool` | HTTP, email, transform nodes | Internal only |
| `postgres` | 3 databases | Internal only |
| `rabbitmq` | Job queues | Internal only |
| `caddy` (optional) | HTTPS reverse proxy | Yes (80/443) |

---

## Prerequisites

- **VPS** (2 GB RAM minimum recommended): Ubuntu 22.04+, Debian 12+, or similar
- **Docker** 24+ and **Docker Compose** v2
- **Domain** (optional): A record pointing to your server for HTTPS
- **SendGrid API key** (optional): only if you use the Email node

---

## Step 1 — Server setup

SSH into your server:

```bash
sudo apt update && sudo apt install -y git ca-certificates curl
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# log out and back in so docker group applies
```

---

## Step 2 — Clone and configure

```bash
git clone https://github.com/itsokAsh/Loom.git
cd Loom
cp .env.example .env
```

Edit `.env` and set **production secrets** (do not use dev defaults):

```bash
nano .env
```

| Variable | Purpose |
|----------|---------|
| `POSTGRES_PASSWORD` | Database password |
| `ADMIN_API_KEY` | Protects `/v1` management API + embedded in UI build |
| `SERVICE_TOKEN` | Worker ↔ trigger-api internal auth |
| `CREDENTIALS_ENCRYPTION_KEY` | Encrypts stored SendGrid credentials (32+ chars) |
| `TRIGGER_PUBLIC_URL` | Public API base with `/v1`, e.g. `https://loom.example.com/v1` |
| `SENDGRID_API_KEY` | Optional, for Email node |
| `DEPLOY_DOMAIN` | Required only for HTTPS profile, e.g. `loom.example.com` |

Generate random secrets:

```bash
openssl rand -hex 32   # run 4 times for different keys
```

**Important:** `ADMIN_API_KEY` is baked into the UI at **build time**. After changing it, rebuild the UI:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml build ui --no-cache
```

---

## Step 3 — Deploy (HTTP on port 3000)

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Wait ~60 seconds for migrations and health checks:

```bash
docker compose ps
docker compose logs -f trigger-api --tail 30
```

Open: **http://YOUR_SERVER_IP:3000**

Verify API through the UI proxy:

```bash
curl -s http://YOUR_SERVER_IP:3000/v1/templates \
  -H "X-Admin-API-Key: YOUR_ADMIN_API_KEY" | head
```

---

## Step 4 — Deploy with HTTPS (recommended for portfolio)

1. Point DNS: `loom.yourdomain.com` → server IP
2. Set in `.env`:
   ```env
   DEPLOY_DOMAIN=loom.yourdomain.com
   TRIGGER_PUBLIC_URL=https://loom.yourdomain.com/v1
   ```
3. Start with Caddy:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  -f deploy/docker-compose.tls.yml \
  --profile tls \
  up -d --build
```

Caddy obtains a Let's Encrypt certificate automatically. Open: **https://loom.yourdomain.com**

---

## Step 5 — Post-deploy smoke test

1. **Gallery** → **Call an API** → Save → Run → **Results** (JSON output)
2. **API Health Check** → set URL → cron `*/5 * * * *` → Start schedule → Results auto-updates
3. **Webhook to HTTP** → Create webhook URL → POST with HMAC (see below)

Webhook test (replace values):

```bash
# Create workflow + webhook in UI first, then:
BODY='{"hello":"loom"}'
SECRET='your-webhook-secret-from-ui'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')
curl -X POST "https://loom.yourdomain.com/v1/webhooks/YOUR_PATH" \
  -H "Content-Type: application/json" \
  -H "X-Signature: sha256=$SIG" \
  -H "Idempotency-Key: deploy-test-1" \
  -d "$BODY"
```

---

## Firewall

Only expose what you need:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 3000/tcp    # HTTP UI (skip if using Caddy only)
sudo ufw allow 80/tcp      # Caddy HTTP
sudo ufw allow 443/tcp     # Caddy HTTPS
sudo ufw enable
```

Do **not** expose Postgres (5440), RabbitMQ (5672/15672), or Redis (6379) publicly. The prod compose overlay removes those port bindings.

---

## Local deploy (Windows / dev machine)

From the repo root in PowerShell:

```powershell
.\scripts\deploy.ps1
```

For production overlay locally:

```powershell
.\scripts\deploy.ps1 -Production
```

---

## Operations

**View logs**

```bash
docker compose logs -f ui trigger-api orchestration-engine node-worker-pool
```

**Restart after code pull**

```bash
git pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

**Backup Postgres**

```bash
docker exec workflow_postgres pg_dumpall -U postgres > loom-backup.sql
```

**Stop**

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml down
```

**Reset everything (destroys data)**

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml down -v
```

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| UI loads but API 401 | `ADMIN_API_KEY` in `.env` must match UI build; rebuild `ui` |
| UI 502 on `/v1` | Wait for `trigger-api` healthy: `docker compose ps` |
| Migrations failed | See [`docs/TROUBLESHOOTING.md`](TROUBLESHOOTING.md) |
| Webhook URL wrong in gallery | Set `TRIGGER_PUBLIC_URL` to your public `https://…/v1` |
| Email node fails | Set `SENDGRID_API_KEY` or add credential in UI |

---

## Files reference

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Base stack (local + prod) |
| `docker-compose.prod.yml` | Secrets, no internal port exposure, required env |
| `deploy/docker-compose.tls.yml` | Hide UI port when using Caddy |
| `deploy/Caddyfile` | HTTPS reverse proxy to UI |
| `.env.example` | Template for all variables |
| `scripts/deploy.sh` | One-command deploy on Linux |
| `scripts/deploy.ps1` | One-command deploy on Windows |

More UI detail: [`UI_DEPLOY.md`](UI_DEPLOY.md)
