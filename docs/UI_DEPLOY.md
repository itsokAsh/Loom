# Loom UI Deployment

## Docker Compose (full stack)

```bash
cp .env.example .env
# Set SENDGRID_API_KEY and ADMIN_API_KEY
docker compose up -d --build
```

- **UI:** http://localhost:3000
- **API:** proxied at http://localhost:3000/v1 (nginx → trigger-api)

## Local development

```bash
docker compose up -d postgres rabbitmq trigger-api orchestration-engine node-worker-pool
cd ui && npm install && npm run dev
```

Vite dev server: http://localhost:5173 (proxies `/v1` to localhost:8080)

## Environment variables

| Variable | Service | Purpose |
|----------|---------|---------|
| `ADMIN_API_KEY` | trigger-api | Management API auth (required in production) |
| `SENDGRID_API_KEY` | node-worker-pool | Email delivery |
| `ORCHESTRATION_URL` | trigger-api | Default `http://orchestration-engine:8081` |
| `VITE_ADMIN_API_KEY` | ui build | Embedded in static bundle for v1 self-hosted |

## Production notes

- Full guide: [`docs/DEPLOY.md`](DEPLOY.md) (VPS, secrets, HTTPS with Caddy).
- Set strong `ADMIN_API_KEY`; default `dev-admin-key` is for local use only.
- Rebuild `ui` after changing `ADMIN_API_KEY` (baked into the static bundle).
- UI contains no mock data — all workflows and executions come from the API.
- Migrations run automatically on `docker compose up` (including `000003` / `000004`).
