#!/usr/bin/env bash
#
# test-seed-gitops-mock.sh — behavioral tests for scripts/seed-gitops-mock.sh.
#
# Runs the seed script against a throwaway bare repo (never touches tmp/gitops-mock.git)
# and asserts:
#   1. idempotency      — running the seed twice leaves exactly one commit
#   2. file structure   — the three expected HelmRelease paths are present
#   3. YAML contract    — every workload exposes spec.values.<w>.image.repository + .tag
#                         (the exact fields internal/gitops/discover.go reads)
#   4. reset semantics  — a fresh bare repo re-seeds cleanly
#
# Requires: bash 3.2+, git, python3 + pyyaml (YAML contract checks skip gracefully without it)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEED_SCRIPT="$REPO_ROOT/scripts/seed-gitops-mock.sh"

PASS=0
FAIL=0
ok()   { echo "  ✅ $1"; PASS=$((PASS+1)); }
fail() { echo "  ❌ $1"; FAIL=$((FAIL+1)); }

EXPECTED_PATHS=(
  "apps/staging/platform-api/platform-api-helmrelease.yaml"
  "apps/production/platform-api/platform-api-helmrelease.yaml"
  "apps/staging/data-pipeline/data-pipeline-helmrelease.yaml"
)

# Run the seed script with tmp/gitops-mock.git pointing at a throwaway sandbox.
# The script resolves the bare repo relative to its cwd, so we run it from a sandbox root.
seed_into() {
  local sandbox="$1"
  ( cd "$sandbox" && bash "$SEED_SCRIPT" )
}

SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT

echo ""
echo "🔎 seed-gitops-mock.sh behavioral tests"
echo "========================================"

# --- Fresh seed + idempotency ------------------------------------------------
mkdir -p "$SANDBOX/tmp"
git init --quiet --bare "$SANDBOX/tmp/gitops-mock.git"

echo ""
echo "▶ first seed"
seed_into "$SANDBOX" >/dev/null
COMMITS=$(git -C "$SANDBOX/tmp/gitops-mock.git" rev-list --count HEAD)
[ "$COMMITS" = "1" ] && ok "one commit after first seed" || fail "expected 1 commit, got $COMMITS"

echo ""
echo "▶ second seed (idempotency)"
seed_into "$SANDBOX" >/dev/null
COMMITS=$(git -C "$SANDBOX/tmp/gitops-mock.git" rev-list --count HEAD)
[ "$COMMITS" = "1" ] && ok "still one commit after re-seed (no-op)" || fail "expected 1 commit, got $COMMITS"

# --- File structure ----------------------------------------------------------
echo ""
echo "▶ file structure"
TREE=$(git -C "$SANDBOX/tmp/gitops-mock.git" ls-tree -r --name-only HEAD)
for p in "${EXPECTED_PATHS[@]}"; do
  echo "$TREE" | grep -qx "$p" && ok "present: $p" || fail "missing: $p"
done

# --- YAML contract (fields discover.go reads) --------------------------------
echo ""
echo "▶ YAML contract (spec.values.<w>.image.repository + .tag)"

# Parallel arrays (bash 3.2 compatible — no declare -A)
YAML_PATHS=(
  "apps/staging/platform-api/platform-api-helmrelease.yaml"
  "apps/production/platform-api/platform-api-helmrelease.yaml"
  "apps/staging/data-pipeline/data-pipeline-helmrelease.yaml"
)
YAML_WORKLOADS=(
  "api worker"
  "api worker"
  "processor"
)

if ! python3 -c 'import yaml' 2>/dev/null; then
  echo "  ⚠ python3/pyyaml not available — skipping YAML contract checks"
else
  for i in "${!YAML_PATHS[@]}"; do
    p="${YAML_PATHS[$i]}"
    content=$(git -C "$SANDBOX/tmp/gitops-mock.git" show "HEAD:$p")
    for w in ${YAML_WORKLOADS[$i]}; do
      if echo "$content" | WL="$w" python3 -c '
import os, sys, yaml
doc = yaml.safe_load(sys.stdin)
w = os.environ["WL"]
img = doc["spec"]["values"][w]["image"]
assert isinstance(img["repository"], str) and img["repository"]
assert isinstance(img["tag"], str) and img["tag"]
' >/dev/null 2>&1; then
        ok "$p → $w has image.repository + image.tag"
      else
        fail "$p → $w missing/invalid image.repository or image.tag"
      fi
    done
  done
fi

# --- Reset semantics ---------------------------------------------------------
echo ""
echo "▶ reset (fresh bare repo re-seeds)"
rm -rf "$SANDBOX/tmp/gitops-mock.git"
git init --quiet --bare "$SANDBOX/tmp/gitops-mock.git"
seed_into "$SANDBOX" >/dev/null
COMMITS=$(git -C "$SANDBOX/tmp/gitops-mock.git" rev-list --count HEAD)
COUNT=$(git -C "$SANDBOX/tmp/gitops-mock.git" ls-tree -r --name-only HEAD | grep -c 'helmrelease.yaml')
{ [ "$COMMITS" = "1" ] && [ "$COUNT" = "3" ]; } && ok "re-seeded: 1 commit, 3 fixtures" || fail "reset failed: commits=$COMMITS fixtures=$COUNT"

echo ""
echo "========================================"
echo "  PASS: $PASS   FAIL: $FAIL"
[ "$FAIL" -eq 0 ] || exit 1
