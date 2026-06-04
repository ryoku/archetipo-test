# KubeGate — Developer Guide

KubeGate is a self-service Kubernetes deployment governance platform. It is a Go + React monolith: a single Go binary that embeds the React SPA as static assets at build time and exposes a versioned REST API (`/api/v1/`) consumed by both the web frontend and a CLI. The stack uses PostgreSQL for persistence and integrates with Keycloak for OIDC authentication.

---

## Directory Structure

```
kubegate/
  cmd/
    server/          API server entrypoint — wires Gin, SPA handler, middleware
    kubegate/        CLI entrypoint (Cobra)
    migrate/         Standalone migration runner
  internal/
    domain/          Domain models (Product, Component, Environment, Deployment, RBAC)
    api/
      handlers/      HTTP handler functions per resource
      middleware/    JWT validation, RBAC enforcement, request logging
      router/        Route registration
    store/           PostgreSQL repository implementations (pgx)
    auth/            OIDC token validation, claims extraction, role mapping
    gitops/          Kustomize patch writer, go-git integration, advisory locking
    gcr/             Google Artifact Registry tag enumeration adapter
    cli/             CLI command implementations
  web/               React + TypeScript SPA (Vite)
    embed.go         Exposes FS (go:embed all:dist) for the server to embed
    src/
      pages/         Page-level components (PascalCase .tsx)
      main.tsx       Entry point
      App.tsx        Router setup
  migrations/        SQL migration files (golang-migrate, sequential numbering)
  docs/              PRD, mockups, test results
  Makefile
  docker-compose.yml Local dev: PostgreSQL 16 + Keycloak
  .env.example       Template for local environment variables
```

---

## Local Dev Setup

### Prerequisites

- Go 1.25+
- Node.js 20+
- pnpm 10+ (`npm install -g pnpm` or see [pnpm.io](https://pnpm.io/installation))
- Docker + Docker Compose
- `air` for Go hot reload (optional — `go run` is the fallback)

### First-time setup

```bash
cp .env.example .env       # fill in any local overrides if needed
make dev                   # starts everything
```

`make dev` does the following in order:

1. Starts Docker Compose (PostgreSQL on `:5433`, Keycloak on `:8080`) and waits until healthy.
2. Initialises the bare gitops mock repo at `tmp/gitops-mock.git` if it does not exist.
3. Runs `pnpm install` in `web/` (offline-preferred).
4. Starts the Vite dev server on `:5173` in the background.
5. Starts the Go server on `:8081` (via `air` if available, else `go run ./cmd/server`).

The Vite dev server proxies `/api/*` to `:8081`, so there are no CORS issues during development.

### Stop the dev environment

```bash
make dev-stop          # stops containers, preserves volumes
make dev-stop-clean    # stops containers and removes volumes (resets DB)
```

---

## Makefile Targets

Targets are organised in three groups: **aggregate** (run both sides), **`go:*`** (backend only), and **`web:*`** (frontend only).

### Aggregate

| Target | Description |
|---|---|
| `make all` | Runs `fmt` → `lint` → `build` → `test` (`go:test`) |
| `make build` | Builds backend (`go:build`) and SPA (`web:build`) |
| `make lint` | Runs `go:lint` and `web:lint` |
| `make test` | Runs `go:test` and `web:test` |
| `make coverage` | Runs `go:coverage` and `web:coverage` |
| `make fmt` | Alias for `go:fmt` |
| `make clean` | Runs `go:clean` and `web:clean` |

### Go

| Target | Description |
|---|---|
| `make go:build` | Compiles `bin/server` and `bin/kubegate` with `-tags prod` |
| `make go:test` | Runs `go test -race -coverprofile=coverage.out -covermode=atomic ./...` |
| `make go:coverage` | Runs `go:test` then generates `coverage.html` and prints the total coverage |
| `make go:lint` | Runs `golangci-lint run ./...` (config in `.golangci.yml`) |
| `make go:fmt` | Runs `gofmt -w` across the repo (excludes `vendor/`, `web/node_modules/`, `tmp/`) |
| `make go:tidy` | Runs `go mod tidy` |
| `make go:clean` | Removes `bin/` and `coverage.out` |

### Web

| Target | Description |
|---|---|
| `make web:install` | Runs `pnpm install --frozen-lockfile` in `web/` (dependency of the other web targets) |
| `make web:build` | Builds the React SPA to `web/dist/` |
| `make web:lint` | Runs `pnpm lint` (ESLint) |
| `make web:test` | Runs `pnpm test` (Vitest) |
| `make web:coverage` | Runs `pnpm test:coverage` (Vitest with v8 coverage, output in `web/coverage/`) |
| `make web:clean` | Removes `web/dist/` and `web/coverage/` |

### Dev / Ops

| Target | Description |
|---|---|
| `make dev` | Starts the full local dev stack (see above) |
| `make dev-stop` | Stops Docker Compose containers |
| `make dev-stop-clean` | Stops containers and removes volumes |
| `make dev-smoke` | Runs `scripts/smoke-dev.sh` against the running dev stack |
| `make migrate` | Sources `.env` and runs `go run ./cmd/migrate -direction up` to apply pending migrations |
| `make migrate-down` | Rolls back one migration step |
| `make sonar` | Runs `sonar-scanner` |

---

## Go Coding Conventions

### Error handling

Wrap errors with context at every layer boundary:

```go
if err := store.Create(ctx, product); err != nil {
    return fmt.Errorf("create product: %w", err)
}
```

- Use `log.Fatal` only in `main()`. Library packages must return errors, never call `log.Fatal`.
- Never swallow errors silently with a blank `_` assignment unless the discard is explicitly intentional and annotated.

### Package naming

- Singular lowercase: `store`, `auth`, `domain`, `gitops`, `gcr`.
- No `util`, `common`, `helpers`, or `shared` packages. Place shared types in `domain`.

### No package-level mutable state

- All state is wired through function parameters or struct fields.
- No package-level variables that change at runtime.
- No `init()` functions.
- `gin.Engine`, database pools, and config structs are created in `main()` and passed down.

### Dependency direction

`cmd/` → `internal/*` → `internal/domain`. Domain types must not import infrastructure packages (`store`, `gitops`, etc.). Infrastructure packages implement domain interfaces.

---

## Frontend Conventions

### Component naming

- Component files: PascalCase, `.tsx` extension.
- Page-level components: `web/src/pages/` (one file per route, e.g. `HomePage.tsx`, `LoginPage.tsx`).
- Shared UI components (when created): `web/src/components/`.

### API calls

All `fetch` calls must go through a dedicated API client module under `web/src/api/`. Never call `fetch` directly from a component. The `web/src/api/` module does not yet exist — when you add the first API call, create it there.

### Routing

Use React Router (`react-router-dom`) for all navigation. Do not manipulate `window.location` directly.

### Styles

Plain CSS files (e.g. `App.css`, `index.css`) or CSS modules. No CSS-in-JS.

---

## Database Migrations

Migrations use [golang-migrate](https://github.com/golang-migrate/migrate) with SQL files. Each migration is a pair of files in `migrations/`:

```
migrations/
  000001_create_products.up.sql
  000001_create_products.down.sql
```

**To add a new migration:**

1. Determine the next sequential number (zero-padded to 6 digits).
2. Create two files:
   - `migrations/NNNNNN_describe_the_change.up.sql` — applies the change
   - `migrations/NNNNNN_describe_the_change.down.sql` — fully reverts it
3. Run `make migrate` to apply.
4. Test the rollback: `make migrate-down`.

Never edit an already-applied migration. Always create a new one.

---

## Test Strategy

### Go unit tests

```bash
make go:test     # or: go test -race ./...
```

All packages. Integration tests guard on `DATABASE_URL` and skip automatically when it is not set:

```go
dsn := os.Getenv("DATABASE_URL")
if dsn == "" {
    t.Skip("DATABASE_URL not set — skipping integration test")
}
```

### Go integration tests

Require a running PostgreSQL instance. Start it with `docker compose up postgres -d` (or `make dev`), then set `DATABASE_URL` from `.env`.

### Frontend type check

```bash
cd web && pnpm exec tsc --noEmit
```

### Frontend e2e (Playwright)

```bash
cd web && pnpm e2e
```

Runs against the built `web/dist/` served by `pnpm serve` (sirv on `:4173`). Build the frontend first if `web/dist/` is stale:

```bash
cd web && pnpm build && pnpm e2e
```

E2e test files live in `web/e2e/`. Demo scenario tests are named `demo__*.spec.ts`. Video recording is currently disabled globally (`video: 'off'` in `web/playwright.config.ts`); the demo test file opts in per-test with `test.use({ video: 'on' })`. Test output is written to `docs/test-results/US-004/` (the `outputDir` in `playwright.config.ts` is per-suite — update it when adding a new spec's e2e suite).

### Static handler unit tests

```bash
go test ./cmd/server/ -v
```

Covers the SPA static file handler with in-memory `fstest.MapFS` fixtures. No Node.js or Playwright required.

### Test coverage

New code must achieve 80% coverage on backend and frontend.

---

## Architectural Style

KubeGate is a **layered monolith**:

- `cmd/` — entrypoints only; wire dependencies and start the process.
- `internal/` — all business logic; no direct package imports from `cmd/` back into `internal/` (one-way).
- Domain types are plain Go structs in `internal/domain/`, not ORM models.
- The REST API is versioned at `/api/v1/` (Gin routes registered in `internal/api/router/`).
- The React SPA is embedded in the server binary at build time via `go:embed all:dist` in `web/embed.go`. The Gin router falls back to `index.html` for any path not matching a static file, enabling client-side routing.
- In dev mode, Vite runs on `:5173` and the Go server on `:8081`. The Vite `server.proxy` config forwards `/api` to `:8081`.
