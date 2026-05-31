#!/usr/bin/env bash
set -euo pipefail

PASS=0
FAIL=0

ok()   { echo "  ✅ $1"; PASS=$((PASS+1)); }
fail() { echo "  ❌ $1"; FAIL=$((FAIL+1)); }

echo ""
echo "🔎 KubeGate dev environment smoke test"
echo "======================================="

# Load env vars from .env if present
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

# 1 — PostgreSQL connectivity
# DB user/name are hardcoded in docker-compose.yml dev values; update here if you change them.
PG_USER=kubegate
PG_DB=kubegate
echo ""
echo "▶ PostgreSQL (port 5433)"
if pg_isready -h localhost -p 5433 -U "${PG_USER}" -d "${PG_DB}" >/dev/null 2>&1; then
  ok "PostgreSQL is accepting connections"
elif command -v nc >/dev/null 2>&1 && nc -z localhost 5433 >/dev/null 2>&1; then
  ok "PostgreSQL port 5433 is open (pg_isready not installed, falling back to nc)"
else
  fail "PostgreSQL is not reachable on localhost:5433"
fi

# 2 — Keycloak reachability (port 8080; curl not in the KC image so we use bash TCP)
echo ""
echo "▶ Keycloak (port 8080)"
if bash -c 'exec 3<>/dev/tcp/localhost/8080' 2>/dev/null; then
  ok "Keycloak is accepting connections on port 8080"
else
  fail "Keycloak is not reachable on localhost:8080"
fi

# 3 — Keycloak realm discovery
echo ""
echo "▶ Keycloak realm 'kubegate'"
REALM="${KEYCLOAK_REALM:-kubegate}"
KEYCLOAK_URL_HOST="${KEYCLOAK_URL:-http://localhost:8080}"
if curl -sf "${KEYCLOAK_URL_HOST}/realms/${REALM}" >/dev/null 2>&1; then
  ok "Realm '${REALM}' OIDC discovery endpoint is reachable"
else
  fail "Realm '${REALM}' not found at ${KEYCLOAK_URL_HOST}/realms/${REALM} (realm may still be importing)"
fi

# 4 — Gitops mock repository
echo ""
echo "▶ Gitops mock repository"
GITOPS_PATH="${GITOPS_REPO_PATH:-./tmp/gitops-mock.git}"
if [ -d "${GITOPS_PATH}" ] && git -C "${GITOPS_PATH}" rev-parse --git-dir >/dev/null 2>&1; then
  ok "Bare git repository exists at ${GITOPS_PATH}"
else
  fail "Gitops mock not found or not a valid bare repo at ${GITOPS_PATH}"
fi

# Summary
echo ""
echo "======================================="
echo "Results: ${PASS} passed, ${FAIL} failed"
echo ""

if [ "${FAIL}" -gt 0 ]; then
  exit 1
fi
