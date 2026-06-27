# US-039 PR Review Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix verified bugs and type-safety issues found during the US-039 PR review, in priority order.

**Architecture:** KubeGate is a Go + React monolith. The fixes span the deploy flow (Go handler), admin activity panel (React), deployment history UI (React), TypeScript API types, and a new background goroutine for stale-record cleanup. Each task is independently testable and committable.

**Tech Stack:** Go 1.25, Gin, pgx/v5, React 18, TypeScript, Vitest, Testing Library

## Global Constraints

- Go error wrapping: `fmt.Errorf("context: %w", err)` at every layer boundary
- No `log.Fatal` outside `main()`
- All state passed via function params / struct fields — no package-level mutable state
- Frontend API calls must go through `web/src/api/` — never `fetch` directly in components
- New code must achieve 80% test coverage (backend and frontend)
- Run `make go:test` after every Go change; run `cd web && pnpm test` after every frontend change
- Commit after each task passes its tests

---

## File Map

| File | What changes |
|------|-------------|
| `internal/api/handlers/deploy.go` | Task 1: abort on Create failure; improve log |
| `internal/api/handlers/deploy_test.go` | Task 1: rename + update test |
| `web/src/pages/AdminPage.tsx` | Task 2: add activityError state + render |
| `web/src/pages/AdminPage.test.tsx` | Task 2: add error-state tests |
| `web/src/api/products.ts` | Task 3: add `'in_progress'` to Deployment union; fix ActivityEvent.error_message |
| `web/src/pages/HistoryPage.tsx` | Task 3: add in_progress case to OutcomeBadge |
| `web/src/pages/HistoryPage.css` | Task 3: add `.hist-outcome--in-progress` styles |
| `cmd/server/main.go` | Task 4: add background stale sweep goroutine |
| `internal/api/handlers/admin.go` | Task 4: remove MarkStaleInProgress from GetActivity; remove staleDuration field |
| `internal/api/router/admin.go` | Task 4: update RegisterAdminRoutes (remove staleDuration arg) |
| `internal/api/handlers/admin_test.go` | Task 4: update setupAdminRouter; add MarkStaleError test |
| `internal/domain/deployment.go` | Task 5: introduce DeploymentOutcome named type |
| `internal/store/deployment_store.go` | Task 5: update interface + impl signature |
| `internal/api/handlers/deploy.go` | Task 5: update updateOutcome helper signature |
| `internal/api/handlers/deploy_test.go` | Task 5: update mock + test variable types |
| `internal/api/handlers/admin_test.go` | Task 5: update mock method signature |

---

## Task 1: Abort Deploy on in-progress Create failure (Critical C1 + Important C2)

**Problem:** When `deploymentStore.Create` fails, `inProgressID` stays `""`, execution falls through to `applyGitOps`, and the handler returns `202 {"deployment_id": ""}`. Gitops ran but there is no audit record, and the caller has no usable ID.

**Files:**
- Modify: `internal/api/handlers/deploy.go:130-134`
- Modify: `internal/api/handlers/deploy.go:278-287` (updateOutcome log)
- Modify: `internal/api/handlers/deploy_test.go` (rename test, add gitops-not-called assertion)

- [ ] **Step 1: Update the Create error block in deploy.go**

  Replace lines 130–134 (the `if err / else` block):

  ```go
  // BEFORE:
  if err := h.deploymentStore.Create(c.Request.Context(), inProgressRecord); err != nil {
      log.Printf("deploy: create in_progress record product=%s env=%s: %v", productSlug, environmentID, err)
  } else {
      inProgressID = inProgressRecord.ID
  }

  // AFTER:
  if err := h.deploymentStore.Create(c.Request.Context(), inProgressRecord); err != nil {
      log.Printf("deploy: create in_progress record product=%s env=%s: %v", productSlug, environmentID, err)
      c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
      return
  }
  inProgressID = inProgressRecord.ID
  ```

- [ ] **Step 2: Improve updateOutcome log on the success path (C2)**

  In `updateOutcome` at line ~284, improve the log message to distinguish a stuck-outcome failure from other errors:

  ```go
  func (h *DeploymentHandlers) updateOutcome(ctx context.Context, id, outcome string, commitSHA *string, errorMessage *string) {
      if id == "" {
          return
      }
      if err := h.deploymentStore.UpdateOutcome(ctx, id, outcome, commitSHA, errorMessage); err != nil {
          if outcome == domain.OutcomeSuccess {
              log.Printf("updateOutcome: gitops succeeded but outcome record stuck as in_progress id=%s (will be swept by stale cleaner): %v", id, err)
          } else {
              log.Printf("updateOutcome id=%s outcome=%s: %v", id, outcome, err)
          }
      }
  }
  ```

- [ ] **Step 3: Rename and update the test that validated the old (broken) behaviour**

  In `internal/api/handlers/deploy_test.go`, find `TestDeploy_InProgressCreateError_DeployContinues` and replace it entirely:

  ```go
  func TestDeploy_InProgressCreateError_Returns500(t *testing.T) {
      // Create failure is fatal: gitops must not run without an audit record.
      // mockGitOpsApplier.Apply panics when applyFn is nil, so if gitops ran this test would panic.
      ds := &mockDeploymentStore{
          createFn: func(_ context.Context, _ *domain.Deployment) error {
              return errors.New("db connection lost")
          },
      }
      r := newDeployRouterFull(
          productStoreWithProduct(deployFixtureProduct),
          envStoreWithEnv(deployFixtureEnv),
          acquiringLockStore(),
          ds,
          &mockGitOpsApplier{}, // no applyFn → panics if called
          editorIdentityForDeploy("my-service"),
      )

      w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
          "workload": "main",
          "tag":      "v1.2.3",
      })

      assert.Equal(t, http.StatusInternalServerError, w.Code)
  }
  ```

- [ ] **Step 4: Run tests**

  ```bash
  make go:test
  ```

  Expected: all tests pass. The old `TestDeploy_InProgressCreateError_DeployContinues` is gone; the new `TestDeploy_InProgressCreateError_Returns500` passes.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/api/handlers/deploy.go internal/api/handlers/deploy_test.go
  git commit -m "fix(deploy): abort on in-progress Create failure instead of proceeding gitops"
  ```

---

## Task 2: Activity fetch error state in AdminPage (Critical C3)

**Problem:** `listAdminActivity` errors are swallowed with `console.error` only. Users see the empty-state message "Nessun evento di deployment recente." on every network/auth failure, indistinguishable from a genuinely empty feed. `listAdminProducts` already has correct error handling — the activity section needs the same treatment.

**Files:**
- Modify: `web/src/pages/AdminPage.tsx`
- Modify: `web/src/pages/AdminPage.test.tsx`

- [ ] **Step 1: Add activityError state**

  In `AdminPage.tsx` at line 56, after the `activityLoading` state line, add:

  ```tsx
  const [activityError, setActivityError] = useState<string | null>(null)
  ```

  The import for `useState` is already present.

- [ ] **Step 2: Update fetchActivity to set and clear error state**

  Replace the `fetchActivity` function (lines 63–68):

  ```tsx
  function fetchActivity() {
    listAdminActivity(token)
      .then((data) => {
        if (!cancelled) {
          setActivity(data)
          setActivityError(null)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          console.error('[AdminPage] listAdminActivity failed:', err)
          setActivityError(err instanceof Error ? err.message : 'Failed to load activity feed')
        }
      })
      .finally(() => { if (!cancelled) setActivityLoading(false) })
  }
  ```

- [ ] **Step 3: Render error banner and fix empty-state condition**

  In the activity section JSX (around line 292), replace the loading/empty block:

  ```tsx
  {activityLoading && (
    <div className="admin-activity-empty" data-testid="activity-loading">
      <span>Caricamento…</span>
    </div>
  )}
  {!activityLoading && activityError && (
    <div className="admin-error" data-testid="activity-error">
      {activityError}
    </div>
  )}
  {!activityLoading && !activityError && activity.length === 0 && (
    <div className="admin-activity-empty" data-testid="activity-empty">
      <span>Nessun evento di deployment recente.</span>
    </div>
  )}
  ```

  The `admin-error` CSS class already exists in `AdminPage.css` (used by the products error path). Verify by grepping:
  ```bash
  grep "admin-error" web/src/pages/AdminPage.css
  ```
  If absent, add to `AdminPage.css`:
  ```css
  .admin-error {
    padding: 12px 16px;
    color: #e05555;
    font-size: 13px;
  }
  ```

- [ ] **Step 4: Add tests for error state**

  In `web/src/pages/AdminPage.test.tsx`, in the `activity panel` describe block (or wherever activity tests live), add:

  ```tsx
  it('shows error banner when listAdminActivity rejects with an Error', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockRejectedValue(new Error('listAdminActivity: 500'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('activity-error')).toBeTruthy()
      expect(screen.getByText(/listAdminActivity: 500/)).toBeTruthy()
    })
  })

  it('does not show activity-empty when fetch fails', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockRejectedValue(new Error('network error'))

    renderPage()

    await waitFor(() => {
      expect(screen.queryByTestId('activity-empty')).toBeNull()
    })
  })

  it('shows fallback error text for non-Error activity rejections', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockRejectedValue('unexpected string')

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Failed to load activity feed')).toBeTruthy()
    })
  })

  it('clears error banner when subsequent poll succeeds', async () => {
    mockListAdminProducts.mockResolvedValue([])
    // First call fails, second succeeds
    mockListAdminActivity
      .mockRejectedValueOnce(new Error('transient error'))
      .mockResolvedValueOnce([makeActivity()])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('activity-error')).toBeTruthy()
    })

    // Simulate a re-fetch by directly calling the mock's second value
    // (we can't easily trigger the 30s interval in unit tests, so verify the
    // resolved path clears the error by re-mounting with a success mock)
    cleanup()
    mockListAdminActivity.mockResolvedValue([makeActivity()])
    renderPage()

    await waitFor(() => {
      expect(screen.queryByTestId('activity-error')).toBeNull()
      expect(screen.getByTestId('activity-list')).toBeTruthy()
    })
  })
  ```

- [ ] **Step 5: Run tests**

  ```bash
  cd web && pnpm test -- --reporter=verbose AdminPage
  ```

  All existing tests pass; the 4 new tests pass.

- [ ] **Step 6: Commit**

  ```bash
  git add web/src/pages/AdminPage.tsx web/src/pages/AdminPage.test.tsx web/src/pages/AdminPage.css
  git commit -m "fix(admin): surface activity feed fetch errors instead of showing empty state"
  ```

---

## Task 3: TypeScript type fixes + HistoryPage OutcomeBadge (Important #4, #7, #8)

**Problem:**
- `Deployment.outcome` is typed `'success' | 'failure'` but `in_progress` records are now returned by `ListByProduct`. Any component switching on this field will silently mishandle them.
- `ActivityEvent.error_message` is `?: string | null` — optional AND nullable, creating three states where two are intended.
- `OutcomeBadge` in `HistoryPage.tsx` has no `'in_progress'` case — it falls through to the failure branch, showing a red badge and failure row tint for in-flight deploys.

**Files:**
- Modify: `web/src/api/products.ts:242`
- Modify: `web/src/api/products.ts:316`
- Modify: `web/src/pages/HistoryPage.tsx:11-30,74`
- Modify: `web/src/pages/HistoryPage.css`

- [ ] **Step 1: Add `'in_progress'` to the Deployment outcome union**

  In `web/src/api/products.ts` line 242, change:

  ```ts
  // BEFORE:
  outcome: 'success' | 'failure'

  // AFTER:
  outcome: 'success' | 'failure' | 'in_progress'
  ```

- [ ] **Step 2: Fix ActivityEvent.error_message dual optionality**

  In `web/src/api/products.ts` line 316, change:

  ```ts
  // BEFORE:
  error_message?: string | null

  // AFTER:
  error_message?: string
  ```

  The backend does not send `error_message` at all for non-failure outcomes (it is `nil`, which marshals as JSON `null` — but `listAdminActivity`'s `res.json()` cast means absent-vs-null is API-determined). Verify the Go handler in `internal/api/handlers/admin.go` — `activityResponse.ErrorMessage` has no `omitempty`, so it serializes as `null` for success/in_progress. Two options:
  - Add `omitempty` to `activityResponse.ErrorMessage` in admin.go (align with deploymentResponse in history.go) and use `error_message?: string` here.
  - Keep null serialization and use `error_message: string | null` (not optional).

  **Go with option 1** (omitempty): add `,omitempty` to admin.go's ErrorMessage tag:
  ```go
  // internal/api/handlers/admin.go line ~57:
  ErrorMessage *string `json:"error_message,omitempty"`
  ```
  Then `error_message?: string` in TypeScript is correct (field absent = no error).

- [ ] **Step 3: Add `'in_progress'` case to OutcomeBadge in HistoryPage.tsx**

  Replace `OutcomeBadge` (lines 11–30):

  ```tsx
  function OutcomeBadge({ outcome }: { readonly outcome: string }) {
    if (outcome === 'success') {
      return (
        <span className="hist-outcome hist-outcome--success">
          <svg width="11" height="11" viewBox="0 0 11 11" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M2 5.5l2.5 2.5 4.5-4.5" />
          </svg>
          success
        </span>
      )
    }
    if (outcome === 'in_progress') {
      return (
        <span className="hist-outcome hist-outcome--in-progress">
          in progress
        </span>
      )
    }
    return (
      <span className="hist-outcome hist-outcome--failure">
        <svg width="11" height="11" viewBox="0 0 11 11" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M2.5 2.5l6 6M8.5 2.5l-6 6" />
        </svg>
        failure
      </span>
    )
  }
  ```

- [ ] **Step 4: Fix row class for in_progress (line 74 of HistoryPage.tsx)**

  Change:

  ```tsx
  // BEFORE:
  <tr key={d.id} className={d.outcome === 'failure' ? 'hist-row--failure' : undefined}>

  // AFTER:
  <tr key={d.id} className={d.outcome === 'failure' ? 'hist-row--failure' : d.outcome === 'in_progress' ? 'hist-row--in-progress' : undefined}>
  ```

- [ ] **Step 5: Add CSS for in_progress badge and row**

  In `web/src/pages/HistoryPage.css`, after the existing `.hist-row--failure` block, add:

  ```css
  /* In-progress row tint */
  .hist-row--in-progress td {
    background: rgba(130, 160, 230, 0.04);
  }
  .hist-row--in-progress:hover td {
    background: rgba(130, 160, 230, 0.08);
  }
  ```

  In the outcome badge section (wherever `.hist-outcome--success` and `--failure` are defined), add:

  ```css
  .hist-outcome--in-progress {
    color: #5b82d4;
    background: rgba(91, 130, 212, 0.1);
    border: 1px solid rgba(91, 130, 212, 0.25);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 11px;
    font-weight: 500;
  }
  ```

  First check if existing `.hist-outcome` base styles are defined there; match the pattern of `--success` and `--failure`.

- [ ] **Step 6: Type-check**

  ```bash
  cd web && pnpm exec tsc --noEmit
  ```

  Expected: zero errors.

- [ ] **Step 7: Run tests**

  ```bash
  cd web && pnpm test
  ```

  All tests pass. (No new tests needed for pure type changes; the type check is the verification.)

- [ ] **Step 8: Commit**

  ```bash
  git add web/src/api/products.ts web/src/pages/HistoryPage.tsx web/src/pages/HistoryPage.css internal/api/handlers/admin.go
  git commit -m "fix(types): add in_progress to Deployment union; fix error_message optionality; handle in_progress in OutcomeBadge"
  ```

---

## Task 4: Background stale sweep — decouple from UI read path (Important #5)

**Problem:** `MarkStaleInProgress` is only called from `GetActivity`. If an admin never opens the activity page, `in_progress` records from crashed deploys are never cleaned up. They persist indefinitely in the history page and activity feed.

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/api/handlers/admin.go`
- Modify: `internal/api/router/admin.go`
- Modify: `internal/api/handlers/admin_test.go`

- [ ] **Step 1: Add background sweep goroutine to main.go**

  In `cmd/server/main.go`, immediately before `r.Run(addr)`, add:

  ```go
  // Sweep in_progress deployments older than the stale timeout once per minute.
  staleDur := handlers.StaleDeploymentTimeout()
  go func() {
      ticker := time.NewTicker(time.Minute)
      defer ticker.Stop()
      for range ticker.C {
          if err := deploymentStore.MarkStaleInProgress(context.Background(), staleDur); err != nil {
              log.Printf("stale sweep: mark stale in_progress: %v", err)
          }
      }
  }()
  ```

  `time` is already imported; `context` is already imported.

- [ ] **Step 2: Remove staleDuration from AdminHandlers**

  In `internal/api/handlers/admin.go`:

  1. Remove `staleDuration time.Duration` from `AdminHandlers` struct.
  2. Update `NewAdminHandlers` signature to remove the `staleDuration time.Duration` parameter:
     ```go
     func NewAdminHandlers(s store.ProductStore, ds store.DeploymentStore) *AdminHandlers {
         return &AdminHandlers{store: s, deploymentStore: ds}
     }
     ```
  3. Remove `MarkStaleInProgress` call from `GetActivity` (lines 65–68):
     ```go
     // DELETE these lines:
     if err := h.deploymentStore.MarkStaleInProgress(ctx, h.staleDuration); err != nil {
         log.Printf("GetActivity: mark stale: %v", err)
         // non-fatal: proceed
     }
     ```

  Also remove the `"os"`, `"strconv"` imports if they are now unused (they were used by `StaleDeploymentTimeout`). Keep `StaleDeploymentTimeout` itself — it is still called from `main.go`.

  Check: `strconv` and `os` are used only by `StaleDeploymentTimeout`. Since that function stays, the imports stay.

- [ ] **Step 3: Update RegisterAdminRoutes to not pass staleDuration**

  In `internal/api/router/admin.go`, update the `NewAdminHandlers` call:

  ```go
  // BEFORE:
  h := handlers.NewAdminHandlers(ps, ds, handlers.StaleDeploymentTimeout())

  // AFTER:
  h := handlers.NewAdminHandlers(ps, ds)
  ```

  Remove the `handlers` import of `StaleDeploymentTimeout` from this file if it now has no other references.

- [ ] **Step 4: Update admin_test.go**

  In `internal/api/handlers/admin_test.go`, update `setupAdminRouter`:

  ```go
  func setupAdminRouter(ms *mockAdminStore, ds store.DeploymentStore) *gin.Engine {
      gin.SetMode(gin.TestMode)
      r := gin.New()
      h := handlers.NewAdminHandlers(ms, ds)   // remove 5*time.Minute
      r.GET("/api/v1/admin/products", h.GetAdminProducts)
      r.GET("/api/v1/admin/activity", h.GetActivity)
      return r
  }
  ```

  Add the test for `MarkStaleInProgress` error — proving `GetActivity` still returns data when the sweep (now gone from the handler) would have failed:

  ```go
  func TestGetActivity_MarkStaleError_StillReturnsActivity(t *testing.T) {
      // The stale sweep was removed from GetActivity; this test documents the contract
      // that a MarkStaleInProgress error (now handled in the background sweep) would
      // not affect the activity response. We simulate it by injecting a store that
      // returns an error from MarkStaleInProgress and verifying that GetActivity
      // still returns the activity list with 200.
      deployedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
      ds := &mockActivityStore{
          listActivityResult: []domain.Deployment{
              {
                  ID:          "dep-id-1",
                  ProductSlug: "my-service",
                  Outcome:     domain.OutcomeSuccess,
                  DeployedAt:  deployedAt,
              },
          },
          markStaleErr: errors.New("db timeout"), // non-fatal, not called by GetActivity
      }
      w := doPlain(setupAdminRouter(&mockAdminStore{}, ds), http.MethodGet, "/api/v1/admin/activity")
      assertStatus(t, w, http.StatusOK)

      var resp []map[string]any
      if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
          t.Fatalf("unmarshal: %v", err)
      }
      if len(resp) != 1 {
          t.Errorf("expected 1 item, got %d", len(resp))
      }
  }
  ```

- [ ] **Step 5: Run tests**

  ```bash
  make go:test
  ```

  Expected: all pass.

- [ ] **Step 6: Commit**

  ```bash
  git add cmd/server/main.go internal/api/handlers/admin.go internal/api/router/admin.go internal/api/handlers/admin_test.go
  git commit -m "fix(admin): move stale in_progress sweep to background goroutine in main"
  ```

---

## Task 5: Introduce `DeploymentOutcome` named type (Important #6)

**Problem:** `Deployment.Outcome` and `UpdateOutcome`'s `outcome` parameter are raw `string`. The three constants exist but are not enforced by the compiler — a caller can pass any string, and a typo compiles silently, failing only at the DB constraint.

**Files:**
- Modify: `internal/domain/deployment.go`
- Modify: `internal/store/deployment_store.go` (interface + implementation)
- Modify: `internal/api/handlers/deploy.go` (`updateOutcome` helper)
- Modify: `internal/api/handlers/deploy_test.go` (mock function signature + test variable types)
- Modify: `internal/api/handlers/admin_test.go` (mock method signature)
- Modify: `internal/api/handlers/history_test.go` (if it has updateOutcomeFn — check)

- [ ] **Step 1: Add DeploymentOutcome type to domain/deployment.go**

  Replace the constants and `Outcome` field:

  ```go
  // DeploymentOutcome is the result of a deployment operation.
  type DeploymentOutcome string

  const (
      OutcomeSuccess    DeploymentOutcome = "success"
      OutcomeFailure    DeploymentOutcome = "failure"
      OutcomeInProgress DeploymentOutcome = "in_progress"
  )

  // Deployment records a single gitops apply operation.
  type Deployment struct {
      ID               string
      ProductID        string
      ProductSlug      string // empty string if not populated by JOIN
      EnvironmentID    string
      ActorDisplayName string
      ComponentName    string
      EnvironmentName  string
      Tag              string
      DeployedAt       time.Time
      CommitSHA        *string           // nil when the deploy failed before a commit was created
      Outcome          DeploymentOutcome // "success" | "failure" | "in_progress"
      ErrorMessage     *string           // non-nil only when Outcome is "failure"
  }
  ```

- [ ] **Step 2: Update DeploymentStore interface**

  In `internal/store/deployment_store.go`, change the `UpdateOutcome` interface line:

  ```go
  // BEFORE:
  UpdateOutcome(ctx context.Context, id, outcome string, commitSHA *string, errorMessage *string) error

  // AFTER:
  UpdateOutcome(ctx context.Context, id string, outcome domain.DeploymentOutcome, commitSHA *string, errorMessage *string) error
  ```

- [ ] **Step 3: Update pgxDeploymentStore.UpdateOutcome implementation**

  ```go
  func (s *pgxDeploymentStore) UpdateOutcome(ctx context.Context, id string, outcome domain.DeploymentOutcome, commitSHA *string, errorMessage *string) error {
      tag, err := s.pool.Exec(ctx,
          `UPDATE deployments SET outcome = $2, commit_sha = $3, error_message = $4 WHERE id = $1`,
          id, outcome, commitSHA, errorMessage,
      )
      // ... rest unchanged
  ```

  pgx handles named string types transparently — `domain.DeploymentOutcome` serializes as its underlying string value.

- [ ] **Step 4: Update updateOutcome helper in deploy.go**

  ```go
  // BEFORE:
  func (h *DeploymentHandlers) updateOutcome(ctx context.Context, id, outcome string, ...) {

  // AFTER:
  func (h *DeploymentHandlers) updateOutcome(ctx context.Context, id string, outcome domain.DeploymentOutcome, ...) {
  ```

  The comparison `outcome == domain.OutcomeSuccess` added in Task 1 now type-checks correctly.

- [ ] **Step 5: Update mockDeploymentStore in deploy_test.go**

  Change the `updateOutcomeFn` field and `UpdateOutcome` method:

  ```go
  updateOutcomeFn func(ctx context.Context, id string, outcome domain.DeploymentOutcome, commitSHA *string, errorMessage *string) error

  func (m *mockDeploymentStore) UpdateOutcome(ctx context.Context, id string, outcome domain.DeploymentOutcome, commitSHA *string, errorMessage *string) error {
      if m.updateOutcomeFn != nil {
          return m.updateOutcomeFn(ctx, id, outcome, commitSHA, errorMessage)
      }
      return nil
  }
  ```

  Update test variables that capture `d.Outcome` — they must be `domain.DeploymentOutcome`, not `string`:

  In `TestDeploy_Success_RecordsDeployment` (and any similar tests):
  ```go
  var createdOutcome domain.DeploymentOutcome
  var updatedOutcome domain.DeploymentOutcome
  // ...
  createFn: func(_ context.Context, d *domain.Deployment) error {
      createdOutcome = d.Outcome  // now domain.DeploymentOutcome
      ...
  },
  updateOutcomeFn: func(_ context.Context, _, outcome domain.DeploymentOutcome, ...) error {
      updatedOutcome = outcome
      ...
  },
  ```

  Search for all occurrences of `var createdOutcome string` and `var updatedOutcome string` in `deploy_test.go` and change them to `domain.DeploymentOutcome`.

- [ ] **Step 6: Update mockActivityStore in admin_test.go**

  ```go
  func (m *mockActivityStore) UpdateOutcome(_ context.Context, _ string, _ domain.DeploymentOutcome, _ *string, _ *string) error {
      return nil
  }
  ```

- [ ] **Step 7: Check history_test.go for any mock that needs updating**

  ```bash
  grep -n "UpdateOutcome\|updateOutcomeFn\|outcome string" internal/api/handlers/history_test.go
  ```

  Update any occurrences found, following the same pattern as Step 5.

- [ ] **Step 8: Verify admin.go response struct**

  In `admin.go`, `activityResponse.Outcome` is typed `string`. The assignment `Outcome: d.Outcome` now assigns a `domain.DeploymentOutcome` to a `string` field — add an explicit cast:

  ```go
  Outcome: string(d.Outcome),
  ```

  Similarly check `history.go` if it has a comparable `deploymentResponse.Outcome = d.Outcome` pattern and apply the same cast.

- [ ] **Step 9: Build check**

  ```bash
  go build ./...
  ```

  Expected: zero errors.

- [ ] **Step 10: Run tests**

  ```bash
  make go:test
  ```

  Expected: all pass.

- [ ] **Step 11: Commit**

  ```bash
  git add internal/domain/deployment.go internal/store/deployment_store.go \
          internal/api/handlers/deploy.go internal/api/handlers/deploy_test.go \
          internal/api/handlers/admin_test.go internal/api/handlers/admin.go
  # Add history.go/history_test.go if modified
  git commit -m "refactor(domain): introduce DeploymentOutcome named type for compile-time safety"
  ```

---

## Self-Review

**Spec coverage:**
- C1 (Create failure abort) → Task 1 ✓
- C2 (updateOutcome log improvement) → Task 1 ✓
- C3 (activity error state) → Task 2 ✓
- #4 (in_progress badge in history) → Task 3 ✓
- #5 (background stale sweep) → Task 4 ✓
- #6 (DeploymentOutcome type) → Task 5 ✓
- #7 (Deployment TS type missing in_progress) → Task 3 ✓
- #8 (error_message dual optionality) → Task 3 ✓

**Placeholder scan:** No TBD/TODO. All steps have exact code.

**Type consistency:**
- `domain.DeploymentOutcome` introduced in Task 5 — `updateOutcome(id string, outcome domain.DeploymentOutcome, ...)` used consistently from Task 1 onward. Tasks 1–4 use string constants; Task 5 makes them typed. Tasks can run in order without type mismatches.
- `activityError` state added in Task 2; `data-testid="activity-error"` used in test assertions of same task.
- `hist-outcome--in-progress` CSS class added in Task 3 Step 5 and referenced in Task 3 Step 3.
