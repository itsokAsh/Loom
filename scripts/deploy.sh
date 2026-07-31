#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PRODUCTION=false
TLS=false

for arg in "$@"; do
  case "$arg" in
    --production|-p) PRODUCTION=true ;;
    --tls) TLS=true ;;
  esac
done

if [[ ! -f .env ]]; then
  echo "Creating .env from .env.example — edit secrets before production deploy."
  cp .env.example .env
fi

FILES=(-f docker-compose.yml)
if [[ "$PRODUCTION" == true ]]; then
  FILES+=(-f docker-compose.prod.yml)
fi
if [[ "$TLS" == true ]]; then
  FILES+=(-f deploy/docker-compose.tls.yml)
  PROFILE=(--profile tls)
else
  PROFILE=()
fi

echo "Deploying Loom (${PRODUCTION:+production }${TLS:+tls })..."
docker compose "${FILES[@]}" "${PROFILE[@]}" up -d --build

echo ""
echo "Done. Check status:"
docker compose "${FILES[@]}" ps
echo ""
if [[ "$TLS" == true ]]; then
  echo "Open https://${DEPLOY_DOMAIN:-your-domain} (set DEPLOY_DOMAIN in .env)"
else
  echo "Open http://localhost:${UI_PORT:-3000}"
fi
