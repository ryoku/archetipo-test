#!/usr/bin/env bash
#
# seed-gitops-mock.sh — pre-seed the local gitops mock repo with demo HelmRelease fixtures.
#
# Idempotent: if the bare repo at tmp/gitops-mock.git already has at least one commit,
# the script exits 0 without modifying history. Otherwise it clones the bare repo into a
# throwaway working tree, writes the demo HelmRelease files, commits, and pushes to `main`.
#
# Invoked automatically by `make dev` (right after the bare repo init step) and by
# `make reset-gitops-mock`.
set -euo pipefail

BARE_REPO="tmp/gitops-mock.git"

# Guard: the bare repo must already exist (created by `make dev` / `make reset-gitops-mock`).
if [ ! -d "$BARE_REPO" ]; then
  echo "❌ $BARE_REPO not found. Run 'make dev' (or 'make reset-gitops-mock') first." >&2
  exit 1
fi

# Idempotency: check for the main branch explicitly (avoids HEAD→master ambiguity on systems
# where init.defaultBranch is not configured to "main").
if git -C "$BARE_REPO" rev-parse --verify refs/heads/main >/dev/null 2>&1; then
  echo "→ gitops mock already seeded — skipping"
  exit 0
fi

REPO_URL="file://$(pwd)/${BARE_REPO}"

WORKTREE="$(mktemp -d)"
trap 'rm -rf "$WORKTREE"' EXIT

git clone --quiet "$REPO_URL" "$WORKTREE" 2>/dev/null

git -C "$WORKTREE" config user.name "KubeGate Seed"
git -C "$WORKTREE" config user.email "seed@kubegate.local"
git -C "$WORKTREE" checkout -B main --quiet

# write_helmrelease <env-slug> <product-slug> <file-content>
write_helmrelease() {
  local env_slug="$1" product_slug="$2" content="$3"
  local dir="$WORKTREE/apps/${env_slug}/${product_slug}"
  mkdir -p "$dir"
  printf '%s' "$content" > "$dir/${product_slug}-helmrelease.yaml"
}

# platform-api / staging — workloads: api, worker
write_helmrelease staging platform-api 'apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: platform-api
  namespace: staging
spec:
  interval: 5m
  chart:
    spec:
      chart: platform-api
      version: "1.x"
      sourceRef:
        kind: HelmRepository
        name: platform-api
        namespace: flux-system
  values:
    api:
      image:
        repository: registry.local/platform-api/api
        tag: v1.4.2
      replicas: 3
    worker:
      image:
        repository: registry.local/platform-api/worker
        tag: v1.4.2
      replicas: 2
'

# platform-api / production — workloads: api, worker (lagging behind staging on purpose)
write_helmrelease production platform-api 'apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: platform-api
  namespace: production
spec:
  interval: 5m
  chart:
    spec:
      chart: platform-api
      version: "1.x"
      sourceRef:
        kind: HelmRepository
        name: platform-api
        namespace: flux-system
  values:
    api:
      image:
        repository: registry.local/platform-api/api
        tag: v1.4.0
      replicas: 5
    worker:
      image:
        repository: registry.local/platform-api/worker
        tag: v1.4.0
      replicas: 3
'

# data-pipeline / staging — workload: processor
write_helmrelease staging data-pipeline 'apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: data-pipeline
  namespace: staging
spec:
  interval: 5m
  chart:
    spec:
      chart: data-pipeline
      version: "0.x"
      sourceRef:
        kind: HelmRepository
        name: data-pipeline
        namespace: flux-system
  values:
    processor:
      image:
        repository: registry.local/data-pipeline/processor
        tag: v0.9.1
      replicas: 2
'

git -C "$WORKTREE" add -A
git -C "$WORKTREE" commit --quiet -m "seed: demo HelmRelease fixtures for platform-api and data-pipeline"
git -C "$WORKTREE" push --quiet origin main

# Point the bare repo's HEAD at main so subsequent clones (go-git writer/discovery) check it out.
git -C "$BARE_REPO" symbolic-ref HEAD refs/heads/main

echo "→ gitops mock seeded with demo HelmRelease fixtures"
