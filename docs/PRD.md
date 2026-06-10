# KubeGate - Product Requirements Document

**Author:** ARchetipo
**Date:** 2026-06-10
**Version:** 1.1

---

## Elevator Pitch

> For **engineering teams in multi-team organizations**, who have the problem of **depending on a DevOps bottleneck to deploy their applications to Kubernetes**, **KubeGate** is a **self-service deployment governance platform** that **empowers product teams to autonomously trigger and manage deployments through governed, auditable, role-scoped workflows backed by FluxCD and gitops**. Unlike **opening tickets to DevOps or giving teams direct access to the gitops repo**, our product **encodes governance rules — tag conventions, environment restrictions, and RBAC — into the system itself, so teams move fast without breaking production**.

---

## Vision

KubeGate transforms Kubernetes deployment from a DevOps-owned manual process into a governed self-service capability available to every product team. By encoding deployment policies — environment access rules, production image tag conventions, product-scoped RBAC — into a platform that speaks directly to FluxCD through the gitops repo, it gives teams the autonomy to ship with confidence while preserving the governance that keeps production safe.

The platform serves multiple teams, potentially across regions, through a web UI for product owners and a CLI for DevOps engineers, exposing all capabilities via a REST API for automation and integration.

### Product Differentiator

KubeGate is not a generic continuous delivery tool. It is a *product-scoped deployment governance layer* that bridges team autonomy and platform reliability. It surfaces only what each user is authorized to see and do, enforces production promotion rules automatically at the server side, and makes the gitops repo an implementation detail rather than a shared surface teams can accidentally break.

---

## User Personas

### Persona 1: Marco

**Role:** Tech Lead / Product Owner
**Age:** 32 | **Background:** 5+ years as a developer and tech lead; comfortable with Docker and basic Kubernetes concepts but not a gitops or infrastructure expert. Focused on feature delivery and release cadence.

**Goals:**
- Deploy his team's services to any environment without waiting for DevOps
- Know at a glance what version is running in each environment
- Promote releases to production confidently, knowing the rules are enforced
- Reduce external team dependencies to increase delivery speed

**Pain Points:**
- Opens a ticket to DevOps for every deployment, even trivial ones to dev/integration
- No visibility into current deployment state across environments without asking DevOps
- Production promotions feel risky and opaque — no guardrails, no audit trail he owns
- Onboarding new team members to the deploy process is ad hoc and error-prone

**Behaviors & Tools:**
Uses Jira, Confluence, and Slack daily. Occasionally uses kubectl for basic debugging. Primarily works in a browser. Does not interact with the gitops repo directly.

**Motivations:** Faster time-to-market, fewer external dependencies, team autonomy, predictable release rhythm.
**Tech Savviness:** Medium — comfortable with web tools and basic CLI; not a gitops or Kubernetes platform expert.

#### Customer Journey - Marco

| Phase | Action | Thought | Emotion | Opportunity |
|---|---|---|---|---|
| Awareness | Hears from manager that a new deployment tool is rolling out | "I hope this actually means I can deploy without a ticket" | Cautiously hopeful | Set clear expectations: self-service for all environments |
| Consideration | Attends onboarding session, sees a live deploy demo | "This looks straightforward — I can do this myself" | Relieved, curious | Emphasize role-based scope: Marco sees only his products |
| First Use | Logs in via SSO, finds his product, lists tags, deploys to dev | "Where is my product? Oh — right there. Tag list works. Done." | Mildly anxious then satisfied | Smooth first deploy flow; deploy success feedback is prominent |
| Regular Use | Deploys to integration before sprint reviews, checks status per environment | "One view tells me what's running where. I stopped asking DevOps weeks ago." | Confident, autonomous | Deploy history gives Marco proof for stakeholders |
| Advocacy | Shows other product owners how to deploy without involving DevOps | "We should have had this years ago." | Proud, empowered | Word-of-mouth adoption across teams |

---

### Persona 2: Sara

**Role:** DevOps / Platform Engineer
**Age:** 28 | **Background:** 4+ years in DevOps and SRE; deep expertise in Kubernetes, FluxCD, Kustomize, and gitops workflows. Owns the platform, the gitops repo, and the CI/CD pipelines.

**Goals:**
- Stop being a deployment bottleneck — define the rules once, enforce them automatically
- Keep the gitops repo clean, consistent, and free from manual team edits
- Maintain full visibility and override capability across all products and environments
- Have a complete, tamper-proof audit trail of every deployment action

**Pain Points:**
- Constant interruptions for trivial deployments that teams could do themselves
- Teams breaking gitops repo structure or YAML formatting when given direct access
- No centralized audit log for who deployed what and when
- Onboarding a new product means manually creating gitops overlays and documenting the process

**Behaviors & Tools:**
Terminal and CLI-first. Writes Go/Python scripts, shell scripts, and K8s manifests. Uses git heavily. Monitors Grafana and Flux dashboards. Reviews PR diffs carefully before merging.

**Motivations:** Platform reliability, team scalability, reduced toil, clean separation of policy from execution.
**Tech Savviness:** High — fluent in Kubernetes internals, gitops patterns, OIDC, and container registries.

#### Customer Journey - Sara

| Phase | Action | Thought | Emotion | Opportunity |
|---|---|---|---|---|
| Awareness | Called in to evaluate KubeGate as the platform owner | "I need to understand exactly how it touches the gitops repo before I trust it" | Skeptical, investigative | Transparent gitops integration docs; Sara can inspect every commit KubeGate makes |
| Consideration | Reviews architecture: HelmRelease patch strategy, locking, tag convention enforcement | "The patch target is `spec.values.[workload].image.tag`, locking is per product-env, regex is server-side. OK." | Cautiously satisfied | ADRs and architecture docs address her specific concerns |
| First Use | Onboards the first product: registers product slug and environment, sets tag convention | "CLI is clean. Config is explicit. I can see exactly what it will commit." | In control | Preview diff before commit builds immediate trust |
| Regular Use | Manages RBAC, monitors deployment history, stops processing deployment tickets | "Zero deployment tickets this week. History log has everything audit needs." | Relieved, productive | Admin dashboard gives Sara full cross-product visibility |
| Advocacy | Defines KubeGate as the standard deployment interface for all new products | "Every new product gets onboarded here on day one — no exceptions." | Confident, authoritative | Product onboarding via CLI reduces Sara's onboarding effort |

---

## Brainstorming Insights

> Key discoveries and alternative directions explored during the inception session.

### Assumptions Challenged

**Assumption:** "DevOps is a bottleneck because they are slow or under-resourced."
**Challenge:** The bottleneck exists by design — because teams deploying directly to the gitops repo without guardrails is dangerous. DevOps is the human policy enforcement layer. The real opportunity is to replace the human gate with a machine gate that is faster, consistent, and auditable. If the governance layer is absent, removing the human gate trades one problem for a worse one.

**Implication for design:** The trust mechanism — tag conventions, RBAC, deployment locking, audit log — is the core of the product, not an add-on. The UX must make these constraints visible and legible, not invisible.

**Assumption:** "Components need to be registered in KubeGate alongside the product."
**Challenge:** Registering workloads in KubeGate duplicates the gitops repo as source of truth and creates drift risk. Since KubeGate already depends on the gitops repo for writes, reading workload structure from the HelmRelease at runtime costs nothing extra and eliminates duplication.

**Implication for design:** KubeGate's domain model is leaner: products and environments are registered; workloads are discovered on-demand. The gitops repo remains the single source of truth for workload configuration.

### New Directions Discovered

- **Regional governance policies:** Different regions may require different production tag conventions or approval flows. Filed as a growth-phase feature: per-region environment configuration.
- **Approval workflows:** For organizations requiring 2-eyes-on-production, a PR-based deploy flow maps naturally onto the gitops model. Filed as post-MVP.
- **Automated promotion pipelines:** In the vision phase, successful CI signals could automatically promote from dev to integration to production, removing humans from the happy path while preserving override capability.
- **Background gitops sync:** Decoupling read operations from real-time git availability via periodic polling sync is viable as a growth-phase optimization, removing the runtime git read dependency for status display.

---

## Product Scope

### MVP - Minimum Viable Product

The MVP goal is to eliminate the DevOps bottleneck for the most common deployment scenarios while establishing the governance layer that makes self-service safe.

- Product and environment definition (CRUD via UI and API); workload discovery on-demand from the gitops repo (no workload registration required in KubeGate)
- GCR image tag enumeration (list available tags per workload, reading the image repository from the HelmRelease, paginated)
- Triggered deployment: patch `spec.values.[workload].image.tag` in the product HelmRelease in the gitops repo
- Per-product-environment deployment lock (prevent concurrent conflict on the gitops repo)
- Tag naming convention enforcement for production environments (configurable regex, global default with per-product override)
- Current deployment state display (what tag is deployed per workload per environment, read from HelmRelease at runtime)
- Deployment history log (actor, workload, tag, environment, timestamp, gitops commit SHA, outcome)
- OIDC authentication via Keycloak
- Product-scoped RBAC: DevOps Admin, Editor, Viewer
- Web UI covering all operations (primary interface for product owners)
- CLI (`kubegate`) covering deploy, status, list-tags, history (primary interface for DevOps)
- Versioned REST API (v1) for all operations, documented in OpenAPI 3.0

### Growth Features (Post-MVP)

- PR-based deployment flow for production environments (create PR instead of direct commit; optional per environment)
- Rollback operation: redeploy a previous tag from the deployment history log
- Real-time FluxCD reconciliation status (pending / synced / failed) via Kubernetes API
- Deployment diff preview before user confirmation
- Azure AD as a secondary OIDC provider (configuration-only change)
- Slack / Teams webhook notifications on deployment events
- Deployment approval workflows (2-eyes for production: requires a second authorized user)
- Per-region environment configuration with region-specific tag conventions
- Background gitops sync: periodic `git pull` to cache workload structure and deployed tag state in PostgreSQL, decoupling read operations from real-time git availability
- Product onboarding templates to accelerate platform engineer setup for new products

### Vision (Future)

- Multi-cluster support (deploy to different Kubernetes clusters per environment)
- Automated promotion pipelines: CI signal triggers dev to integration to production promotion
- SLO / SLA tracking per product and environment
- Integration with change management systems (ServiceNow, Jira Service Management)
- Cost attribution reporting per product and environment
- Self-service product onboarding: product teams register their own products, subject to DevOps Admin approval

---

## Technical Architecture

> **Proposed by:** Leonardo (Architect)

### System Architecture

KubeGate follows a layered monolith pattern: a single Go-based backend exposes both the REST API and shares domain types with the CLI binary. The web frontend is a React SPA served as static assets embedded in the backend binary. This minimizes operational surface area while keeping the codebase navigable.

**Architectural Pattern:** Layered monolith (API + CLI share domain core); SPA frontend consuming REST API

**Main Components:**

1. **API Server** — Go HTTP server (Gin). Handles REST routing, JWT validation, RBAC enforcement, request/response serialization.
2. **CLI** (`kubegate`) — Go binary using Cobra. Shares domain types with the server. Authenticates via OIDC device flow or stored token.
3. **Web Frontend** — React + TypeScript SPA. Consumes the REST API. Embedded in the server binary at build time via `embed`.
4. **Domain Core** — Go packages for Product, Environment, Deployment, and RBAC domain models and business rules. Workloads are not domain entities — they are discovered at runtime from the gitops repo.
5. **GitOps Writer** — Go package using `go-git`. Handles clone/pull, patching `spec.values.[workload].image.tag` in the product HelmRelease at `apps/[env-slug]/[product-slug]/[product-slug]-helmrelease.yaml`, commit, and push. Also reads the HelmRelease at runtime to discover available workloads and current deployed tags. Uses PostgreSQL advisory locks for concurrency control.
6. **GCR Adapter** — Google Artifact Registry API client for tag enumeration. The image repository path is read from `spec.values.[workload].image.repository` in the HelmRelease at runtime.
7. **PostgreSQL** — Persistent store for all domain data: products, environments, RBAC assignments, tag convention rules, deployment history, and deployment locks.
8. **OIDC Middleware** — Validates Keycloak-issued JWTs, extracts user identity and groups, maps to KubeGate roles.

### Technology Stack

| Layer | Technology | Version | Rationale |
|---|---|---|---|
| Language | Go | 1.22+ | K8s ecosystem native; single binary distribution; strong concurrency primitives for locking |
| Backend Framework | Gin | 1.9+ | Lightweight HTTP router; minimal magic; battle-tested in Go API services |
| Frontend Framework | React + TypeScript | 18+ / 5+ | Industry standard SPA; strong typing reduces runtime errors in complex UI state |
| Database | PostgreSQL | 16+ | ACID guarantees for deployment locking; rich advisory lock support; reliable at scale |
| DB Access | pgx + sqlc | 5+ / latest | Type-safe SQL generation; no ORM magic; generated code is auditable |
| Auth | golang-jwt + coreos/go-oidc | - | Standard JWT validation; OIDC discovery for Keycloak and future Azure AD |
| CLI Framework | Cobra | - | De facto standard for Go CLIs; kubectl-style subcommand pattern |
| Frontend Build | Vite | 5+ | Fast dev server; clean production builds; easy embed integration |
| Testing | testify + httptest + testcontainers | - | Unit and integration tests; real Postgres in CI via testcontainers |

### Project Structure

**Organizational pattern:** Domain-driven packages inside `internal/`; entrypoints in `cmd/`; frontend in `web/`

```text
kubegate/
  cmd/
    server/           ← API server entrypoint
    kubegate/         ← CLI entrypoint
  internal/
    domain/           ← Product, Environment, Deployment, RBAC models
    api/
      handlers/       ← HTTP handler functions per resource
      middleware/     ← JWT validation, RBAC enforcement, request logging
      router/         ← Route registration
    gitops/           ← HelmRelease patch writer, go-git integration, locking
    gcr/              ← Google Artifact Registry tag enumeration adapter
    auth/             ← OIDC token validation, claims extraction, role mapping
    store/            ← PostgreSQL repository implementations
    cli/              ← CLI command implementations
  web/                ← React frontend source; built output embedded via go:embed
  migrations/         ← SQL schema migrations
  docs/
    openapi.yaml      ← OpenAPI 3.0 spec
  Makefile
  docker-compose.yml  ← Local dev: PostgreSQL + Keycloak + gitops repo mock
```

### Development Environment

Local development uses Docker Compose to provide PostgreSQL, a Keycloak instance pre-configured with a dev realm, and a bare local git repository acting as the gitops repo mock. A `make dev` target starts all dependencies and the API server with hot reload.

**Required tools:** Go 1.22+, Node.js 20+, Docker + Docker Compose, make, sqlc, air (hot reload)

### CI/CD & Deployment

**Build tool:** Makefile + goreleaser

**Pipeline:** GitHub Actions
- On PR: lint (golangci-lint), unit tests, integration tests (testcontainers), frontend build
- On merge to main: build multi-arch Docker image, push to registry, generate CLI binaries via goreleaser
- On tag (`v*.*.*`): release artifacts published to GitHub Releases

**Deployment:** Helm chart included in repo under `charts/kubegate/`; deployed on Kubernetes

**Target infrastructure:** Kubernetes (any CNCF-conformant cluster; cloud-agnostic)

### Architecture Decision Records (ADR)

| # | Decision | Rationale |
|---|---|---|
| ADR-01 | Go for backend and CLI | Single language across server and CLI; shared domain types; fits K8s ecosystem conventions |
| ADR-02 | HelmRelease values patch only | Mutate only `image.tag` within the target workload's `spec.values` section; never overwrite the full HelmRelease manifest; preserves all other Helm configuration |
| ADR-03 | PostgreSQL advisory locks for deployment serialization | Prevents concurrent gitops repo conflicts; no external lock service needed; locks released on connection drop |
| ADR-04 | Tag convention as server-side configurable regex | Global default with per-product override; enforcement at API layer — not bypassable via CLI or API |
| ADR-05 | Rollback = redeploy previous tag from history | Explicit, auditable, idempotent; avoids silent git reverts that could hide intentional state |
| ADR-06 | Frontend embedded in server binary | One binary, one image, no separate static asset server |
| ADR-07 | OIDC issuer as configuration, not code | Keycloak and Azure AD are both OIDC-compliant; switching providers is a config change, not a code change |
| ADR-08 | Workload discovery at runtime from HelmRelease | KubeGate does not maintain a workload registry; workloads and image repository paths are discovered on-demand by reading `spec.values` from the HelmRelease; the runtime git read dependency is accepted for MVP; background sync is deferred to the growth phase |

---

## Functional Requirements

### Product & Environment Management

| ID | Requirement |
|---|---|
| FR-01 | Create, update, and archive digital products with name, slug, description, and team ownership |
| FR-02 | Workloads within a product are not registered in KubeGate; they are discovered on-demand by reading `spec.values` sections that contain an `image` block in the product's HelmRelease; the image repository path is read from `spec.values.[workload].image.repository` |
| FR-03 | Environments are global platform entities (name, slug, type: dev / integration / production); the gitops path for any product in any environment is derived automatically as `apps/[env-slug]/[product-slug]/[product-slug]-helmrelease.yaml` and is not configurable per product |

### Deployment

| ID | Requirement |
|---|---|
| FR-04 | List available image tags for a workload by reading the image repository from the HelmRelease and querying GCR, with pagination and optional prefix/regex filtering |
| FR-05 | Trigger deployment of a specific image tag for a workload to a target environment, subject to RBAC authorization checks |
| FR-06 | Enforce a configurable tag naming convention (regex) for production-type environments; validate before any gitops mutation; reject non-conforming tags with a clear error message |
| FR-07 | Apply the deployment by patching `spec.values.[workload-name].image.tag` in the HelmRelease at `apps/[env-slug]/[product-slug]/[product-slug]-helmrelease.yaml` and committing the change; acquire a per-product-environment advisory lock before writing; release the lock after commit or on failure |
| FR-08 | (Post-MVP) Support PR-based deployment flow as an optional per-environment configuration: create a PR in the gitops repo instead of a direct commit |
| FR-09 | (Post-MVP) Display a preview diff of the HelmRelease patch that will be committed before the user confirms the deployment |

### Visibility & Audit

| ID | Requirement |
|---|---|
| FR-10 | Show the currently deployed image tag per workload per environment, derived from reading `spec.values.[workload].image.tag` in the HelmRelease for that product/environment from the gitops repo at runtime |
| FR-11 | Maintain an immutable deployment history log: actor, workload, tag, environment, timestamp, gitops commit SHA, and outcome (success / failure with error message) |
| FR-12 | (Post-MVP) Expose FluxCD reconciliation state per environment (pending / synced / failed) by reading Flux Kustomization status from the Kubernetes API |

### Access Control

| ID | Requirement |
|---|---|
| FR-13 | Authenticate all users via OIDC; Keycloak is the primary provider; Azure AD is supported as a secondary provider by changing the OIDC issuer configuration |
| FR-14 | Enforce product-scoped RBAC with three roles: DevOps Admin (all products, full read/write including user management), Editor (assigned products, read/write on product data and deployment operations), Viewer (assigned products, read-only) |
| FR-15 | Manage role assignments (grant / revoke product access and roles for users) via the web UI and REST API; restricted to DevOps Admin role |

### Interfaces

| ID | Requirement |
|---|---|
| FR-16 | Expose all operations via a versioned REST API with `/api/v1/` prefix; documented in OpenAPI 3.0; all endpoints require authentication |
| FR-17 | Provide a CLI (`kubegate`) covering: `deploy`, `list-tags`, `status`, `history`, `product list/create`, `env list/create`; authenticate via OIDC device authorization flow or a stored token |

---

## Non-Functional Requirements

### Security

- All API endpoints require a valid JWT; no anonymous access to any resource
- RBAC is enforced at the API layer on every request; the frontend scope restriction is UX only, not a security boundary
- Tag convention enforcement is executed server-side and cannot be bypassed via the CLI or direct API calls
- Gitops repo credentials (deploy key or OAuth token) are stored encrypted and never written to logs or API responses
- All deployment actions are recorded in the audit log with the authenticated user's identity (sub claim from JWT)
- All web and API traffic requires TLS; HTTP redirects to HTTPS
- Sensitive configuration loaded from environment variables or mounted secrets; never hardcoded

### Integrations

| Integration | Purpose | Notes |
|---|---|---|
| Google Artifact Registry API | Enumerate available image tags per workload | Authenticated via GCP service account or Workload Identity; repository path read from HelmRelease at runtime |
| Keycloak (OIDC) | User authentication and group/role claim extraction | Primary auth provider for MVP |
| Azure AD (OIDC) | User authentication | Post-MVP; same OIDC interface, different issuer config |
| Git (gitops repo) | Read HelmRelease to discover workloads and current state; write image tag patches | HTTPS with token or SSH with deploy key; configurable per installation; runtime read dependency accepted for MVP |
| FluxCD / Kubernetes API | Read reconciliation status per environment | Post-MVP; requires in-cluster service account or kubeconfig |

---

## Open Questions

| # | Question | Default Assumption |
|---|---|---|
| OQ-01 | How should concurrent deployments to the same product-environment be surfaced to the user — queue, reject, or show lock holder? | Reject with a clear error message identifying the lock holder and timestamp |
| OQ-03 | Should deployment history be retained indefinitely or subject to a retention policy? | Indefinite retention in MVP; configurable retention policy in growth phase |
| OQ-04 | Is a global default tag convention regex sufficient for MVP, or do multiple products already use different conventions? | Global default regex with per-product override is the MVP model |

---

## Next Steps

1. **Backlog** - Run `/archetipo-spec` to transform this PRD into a backlog
2. **Design** - Run `/archetipo-design` for UI mockups (when applicable)
3. **Validation** - Review with stakeholders and test the riskiest assumptions

---

_PRD generato via ARchetipo Product Inception - 2026-06-10_
_Sessione condotta da: ianniellor@mediaworld.it con il team ARchetipo_
