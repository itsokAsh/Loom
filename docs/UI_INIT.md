# Loom UI — Agent Memory File

**Repo:** https://github.com/itsokAsh/Loom.git  
**Rule:** Zero mock data — all UI state from API or user input.

## Architecture

| Service | Port | DB |
|---------|------|-----|
| trigger-api | 8080 | trigger_db |
| orchestration-engine | 8081 (health + node API) | orchestration_db |
| node-worker-pool | 8081 internal | worker_db |
| ui (nginx) | 3000 | — |

**Auth:** `X-Admin-API-Key` on all `/v1` management routes (default dev: `dev-admin-key`).

## Node types

| Type | Config fields | Security |
|------|---------------|----------|
| EMAIL | to, from, subject, body, cc, bcc | `from` must not use `{{trigger.` |
| HTTP | method, url, headers, body | `url` must not use `{{trigger.` |
| TRANSFORM | mapping (JSON) | — |
| TRIGGER | UI-only — not saved to worker DAG | — |

Placeholders: `{{trigger.*}}`, `{{config.*}}`, expr in `{{ ... }}`.

## Gallery templates (8)

welcome-email, password-reset, admin-notification, user-onboarding, signup-dual-notify, api-health-check, webhook-relay, vip-conditional

Config keys: sendgrid_from_email, app_name, admin_email, crm_api_url, health_check_url, relay_target_url

## API quick reference

- GET/POST `/v1/workflows`, GET/PATCH/DELETE `/v1/workflows/{id}`
- POST `/v1/workflows/{id}/versions`, GET webhooks/schedules/executions
- GET `/v1/executions/{id}`, GET `/v1/executions/{id}/nodes`
- GET `/v1/templates`, POST `/v1/templates/{id}/create`
- POST `/v1/webhooks/{path}` (HMAC signed trigger)

## Dev commands

```bash
docker compose up -d
cd ui && npm install && npm run dev   # http://localhost:5173, proxies /v1
```

## UI file map

- `ui/src/App.jsx` — shell + workflow context
- `ui/src/api/` — API client
- `ui/src/components/` — canvas, panels, sidebar, gallery
- `ui/src/lib/dagTransform.js` — ReactFlow ↔ Loom DAG

## Status

See plan todos in Cursor — update this file when features land.
