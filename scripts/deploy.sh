#!/usr/bin/env bash
# Build the admin panel and start the production Docker stack.
# Usage (from repo root): ./scripts/deploy.sh   — or: make deploy

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# ── Postgres credentials guard ────────────────────────────────────────────
# docker-compose.yml's ${POSTGRES_USER}/${POSTGRES_PASSWORD} substitution is
# resolved ONLY from a root-level .env (or the shell env) — api/.env is never
# consulted for this. Without a root .env, it silently falls back to
# admin/password. Refuse to deploy on that fallback.
if [[ ! -f "${ROOT_DIR}/.env" ]]; then
  echo "ERROR: ${ROOT_DIR}/.env is missing." >&2
  echo "       docker-compose.yml needs POSTGRES_USER/POSTGRES_PASSWORD/POSTGRES_DB from a root .env" >&2
  echo "       (this is separate from api/.env). Copy .env.example to .env and set a strong password:" >&2
  echo "         cp .env.example .env && sed -i \"s/changeme_use_a_strong_random_value/\$(openssl rand -hex 24)/\" .env" >&2
  exit 1
fi
if grep -qE '^POSTGRES_PASSWORD=(password|changeme_use_a_strong_random_value)?$' "${ROOT_DIR}/.env"; then
  echo "ERROR: ${ROOT_DIR}/.env has a default/placeholder POSTGRES_PASSWORD. Set a real value before deploying." >&2
  exit 1
fi

echo "==> Building admin panel (web/)..."
docker run --rm \
  -v "${ROOT_DIR}/web:/app" \
  -w /app \
  node:20-alpine \
  sh -c "npm install && npm run build"

echo "==> Starting stack (docker compose up -d --build)..."
if [[ ! -f "${ROOT_DIR}/api/secrets/firebase-service-account.json" ]]; then
  echo "WARNING: api/secrets/firebase-service-account.json is missing — push notifications will not be sent." >&2
  echo "         Download from Firebase Console (Design Express project) → Service accounts → Generate new private key." >&2
fi
docker compose up -d --build

echo "==> Deploy complete."
