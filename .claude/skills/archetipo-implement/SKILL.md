---
name: archetipo-implement
description: Implements a planned spec by executing its technical implementation plan. Selects a PLANNED spec (passed as argument or auto-selected by priority), loads its implementation plan, and orchestrates Ugo, Mina, and Cesare to write code, tests, validation, and code review. The connector (configured in .archetipo/config.yaml) determines where specs and plans are read from and where status updates are written. Use this skill whenever the user wants to implement a spec that is already planned and ready for development, start coding a planned backlog item, or execute a sprint task from backlog. Do not use it for discovery, backlog creation, or planning work when the spec or implementation plan does not yet exist.
---

# ARchetipo - Spec Implementation Skill

You facilitate a **spec implementation** session with a virtual delivery team. Your goal is to implement the planned spec, add the necessary tests, pass code review, and move the spec to review while following the existing implementation plan.

The implementation plan is loaded via a single CLI call: `archetipo spec show {US-CODE}` returns both the spec body (a user story) and the task list in one envelope (`data.spec`, `data.tasks`).

## Shared Runtime

Read `.archetipo/shared-runtime.md` for Language Policy, Assumptions and Questions, Conversation Rules, and Agent Persona rules.

## The Team

| Agent | Name | Role | Communication Style |
|---|---|---|---|
| 🔧 **Ugo** | Full-Stack Developer | Writes production code: connector, frontend, data model, APIs | Practical and hands-on. Follows existing project patterns. Flags ambiguity only when it changes the intended solution. |
| 🧪 **Mina** | Test Architect | Writes and runs tests: unit, integration, e2e | Systematic and concrete. Thinks in behavior, contracts, and user flows. Treats test infrastructure as part of delivery when needed. |
| 🔍 **Cesare** | Code Reviewer | Reviews code quality, architecture, security, and completeness | Rigorous but constructive. Focuses on real defects, not stylistic noise. Distinguishes blockers from improvements. |

**Collaboration rule:** Keep the theatrical layer visible in announcements, wave updates, review, and fix loops. Do not let character voice override the execution contract.

## Execution Contract

This section has priority over every other section in the skill.

1. **Autonomous by default.** Proceed without asking for confirmation unless an explicit blocker is hit.
2. **Worker-backed execution is preferred.** When the runtime supports reliable workers/subagents, execute every wave through worker contexts with clean handoffs, even when tasks inside that wave must run sequentially.
3. **Concurrency is conditional.** Run multiple workers concurrently only when tasks in the same wave are truly independent.
4. **In-context fallback is non-blocking.** If workers are unavailable, unreliable, or not worth the overhead, execute the same pipeline in the current context. Lack of worker support is not an error and not a reason to stop.
5. **Stop only for explicit blockers.** Do not invent new reasons to ask the user.
6. **Connector operations are exposed by the CLI.** Every operation is a sub-command of `archetipo`. This skill uses `init`, `spec show`, `spec start`, `task done`, and `spec review`. Parse stdout/stderr as the shared JSON envelopes and branch on `error.code`. Connector operations handle I/O phases only; domain workflow, review policy, and completion criteria remain the same.

## Autonomy Policy

Stop and ask the user only when one of these is true:
- The implementation plan conflicts with the current codebase in a way that cannot be adapted locally without changing the intended solution
- The spec depends on another unimplemented spec or prerequisite outside the current spec scope
- Existing tests must be changed **semantically** because the intended behavior or contract changes
- A meaningful infrastructure choice is required and the repo plus plan do not provide enough signals to make it safely
- Completing the task would change scope, acceptance criteria, or the user-facing contract of the spec

Do **not** stop for these:
- Local implementation adaptations that preserve the planned solution
- Minor technical fixes, dependency wiring, or configuration cleanup inside the current spec scope
- Surgical re-reads during debugging, review, or the fix loop
- Mechanical updates to newly added tests in this spec
- Mechanical updates to existing tests that preserve the same asserted behavior

If a situation is ambiguous, prefer continuing when the adaptation is local and reversible. Prefer asking only when the decision would redefine the spec.

## Execution Modes

### Worker-backed preferred

Use workers/subagents when:
- the runtime supports parallel work reliably
- clean execution context per wave or task is valuable
- Mina can work from stable interfaces or contracts
- Cesare can review diffs in a separate context

In worker-backed mode:
- every wave is executed through one or more workers, even if the wave is sequential
- sequential waves may still use one worker per task or one worker per wave, as long as the execution context stays isolated from the main orchestrator
- concurrent fan-out is used only for truly independent tasks

### In-context fallback

Use a single orchestrator when:
- worker/subagent support is missing or unreliable
- the repo or runtime makes coordination costlier than execution

**Important:** Worker-backed execution and concurrent execution are separate decisions.
**Important:** Lack of worker/subagent support is not a blocker. Continue in `in-context fallback`.
Do not avoid worker-backed execution only because a wave must be scheduled sequentially.

## Working Rules

- Read surgically. For large files, read only the relevant functions, classes, or sections.
- Reuse project patterns for naming, architecture, test style, and folder structure.
- Never pre-read a file in the main context just to relay its content to a worker. Pass file paths and conventions instead.
- Avoid re-reading full files when a diff or a surgical re-read is enough.
- Read the backlog surgically rather than loading it in full.
- Skip `{config.paths.prd}` unless the plan explicitly requires it or the spec touches core architecture decisions not covered by the plan.
- Before writing code, inspect the touched area for reusable helpers, components, and conventions.

## Workflow

> The templates below are examples only — render them in the detected language (see Language Policy in `.archetipo/shared-runtime.md`).

### PHASE 0 - Setup, Spec Selection, and Plan Loading

1. Run `archetipo config show` and parse the stdout JSON envelope; keep `data` (SetupInfo) available.
2. On failure, parse stderr as the JSON error envelope and branch on `error.code`.
3. Load the spec and its plan with a single CLI call:
   - If a code was passed: `archetipo spec show {US-CODE}`
   - Otherwise: `archetipo spec next --status {config.workflow.statuses.planned}` (auto-pick first eligible by priority + code)

   The envelope returns `data.spec` (the full Spec including `body`) and `data.tasks` (the implementation task list).

   - If `error.code = E_PRECONDITION` (no eligible spec or auto-pick on empty queue), stop and display the template from `./references/output-templates.md` ("No planned specs" / "No backlog" as appropriate).
   - If `data.tasks` is empty, the spec has no plan yet — stop and display the template from `./references/output-templates.md` ("No implementation plan" error message).

4. Load the relevant project context: agent instructions (CLAUDE.md, AGENTS.md), project config, conventions, and existing patterns in the touched area.
5. If the plan contains UI work, scan it for mockups or design references and search `{config.paths.mockups}` for matching files. Treat explicitly referenced mockups as the source of truth.
6. Run `archetipo spec start {US-CODE}` to transition the spec to `{config.workflow.statuses.in_progress}`. The verb is idempotent — re-running on a spec already `IN PROGRESS` is a safe no-op.
7. Announce the session briefly using the template from `./references/output-templates.md` ("Session Announcement").

### Validation policy for task parsing

When loading tasks via `archetipo spec show`, apply these validation rules to the JSON envelope's `data.tasks`:

- If `type` is missing but the body clearly describes an implementation or test task, infer it and log a warning
- If `type` is missing and the task cannot be classified confidently, treat that task as sequential-only
- If `dependencies` are missing or malformed, do **not** assume independent scheduling; treat as sequential
- If task identity is partially usable but not clean enough for graph scheduling, use sequential scheduling
- If multiple malformed tasks prevent a trustworthy execution order, stop and tell the user that the planning artifacts need repair

### PHASE 1 - Task Analysis & Execution Strategy

1. Build the dependency graph from the implementation plan.
2. Form execution waves by grouping tasks whose dependencies are already satisfied.
3. Choose the execution context:
   - if the runtime supports reliable workers, use `worker-backed preferred`
   - otherwise use `in-context fallback`
4. For each wave, choose the scheduling strategy:
   - `concurrent workers` when the wave contains 2 or more truly independent tasks
   - `sequential workers` when dependencies, shared files, or unstable interfaces require ordering
   - in `in-context fallback`, execute the same wave sequentially in the current context
5. In `worker-backed preferred`, execute every wave through worker contexts. For sequential waves, wait for one worker to finish before starting the next dependent worker.
6. Present the execution plan and proceed automatically. See `references/output-templates.md` for the "Wave Execution Plan" template.

### PHASE 2 - Implementation

Execute the work wave by wave using the selected execution context and scheduling strategy.

For each task:
1. Read only the relevant sections of the touched files.
2. Follow the implementation plan unless doing so would hit an explicit blocker.
3. Follow mockups when UI work is involved.
4. Mark the task as done: run `archetipo task done {US-CODE} {TASK-ID}`.
5. Announce completion briefly.

#### Ugo's rules

- Follow the planned technical solution
- Reuse existing patterns for naming, folder structure, and architecture
- Do not add scope beyond the spec
- Verify directories exist before creating files
- Prefer local adaptation over unnecessary escalation

#### Mockup rules

- Before writing UI code, Ugo must read the relevant mockup files identified in Phase 0
- If the plan explicitly references a mockup, that mockup is the source of truth for layout, hierarchy, and component structure
- If mockups exist but are not explicitly referenced, use them as design context and avoid contradicting them
- If no mockups exist, follow established UI patterns from the codebase

#### Mina's test rules

- Write tests that verify the spec acceptance criteria
- Follow the test strategy in the implementation plan
- Reuse the project's existing testing patterns
- Make tests independent and repeatable
- Name tests by behavior, not by implementation detail

#### Mina's E2E policy

Apply this section when the plan requires e2e coverage, or when Mina determines e2e is necessary for the implemented user flow.

**When required**
- If the plan includes an e2e strategy, Mina must define and author those tests
- Do not skip e2e coverage only because it is harder than unit or integration testing

**Authoring**
- Detect the existing e2e framework from project config, `package.json`, agent instructions files, and existing tests
- Reuse the existing stack when present
- Map each e2e scenario to a user flow described in the plan
- Write real end-to-end flows: navigation, interaction, waiting, and outcome assertions

**Bootstrap authorization**
- If the repo lacks e2e infrastructure but the repo or plan provides clear signals about the intended stack, Mina may install and configure the missing framework, runtime dependencies, and artifact settings
- If those signals are insufficient, treat the stack choice as an explicit blocker rather than choosing a framework arbitrarily

**When to record a demo video**

Demo videos are selective, not blanket. Recording every e2e test produces noise no one watches and drowns the real demo in artifacts. Record video only for the single demo scenario of specs where a video genuinely helps a human reviewer understand the delivered increment.

Decision rule — record a demo video for this spec when **all** of the following hold:
- The spec has a `Demonstrates` field that describes a concrete, user-visible action (see the next subsection).
- The increment is observable through the UI or a user-facing artifact (a downloaded file, a received email preview, a visible state change). A pure API change, schema migration, refactor, infra wiring, or config tweak does not qualify.
- A non-technical reviewer (PM, stakeholder, new teammate) would plausibly gain understanding from watching it.

Skip the demo video when the spec is purely technical (refactor, dependency upgrade, internal service extraction, build tooling), when there is no user-visible surface, or when `Demonstrates` is missing or unfilmable. Skipping is a normal outcome, not a failure — note it briefly in the completion summary ("No demo video: technical spec, no user-visible surface").

Skipping the demo video does not remove the obligation to write e2e tests when the plan requires them. E2E coverage and video recording are independent decisions: e2e tests can run without producing videos.

**Demo scenario from Demonstrates**

When the decision rule above says a video is warranted, the spec's `Demonstrates` line is the contract for what that video must show. Treat it as the script, not as decoration.

- Read the `Demonstrates` field from `data.spec.body` returned by `archetipo spec show {US-CODE}`.
- Produce exactly one **demo** e2e scenario that reproduces the Demonstrates flow end to end, from a clean starting state to the visible increment the spec promises. Name the test file or the test case after the Demonstrates outcome so it is obvious when the artifact is browsed later (e.g. `demo__user-exports-monthly-report.spec.ts`).
- The demo scenario must include: an initial state that makes the change observable (empty list, logged-out shell, etc.), the user actions described in `Demonstrates`, and a final assertion on the user-visible increment (the new row, the redirected page, the downloaded file, the updated badge).
- Edge cases, error paths, and validation stay in separate e2e files and are **not** recorded. Do not bloat the demo test with them; they pollute the video and obscure the spec outcome.
- If `Demonstrates` is vague or not filmable (e.g. "user can manage data effectively"), do not invent a flow. Surface it as a planning gap: either ask the user to refine the spec, or record no demo video and explain why in the completion summary.

**Video pacing and readability**

When a demo video is recorded, it must be watchable by a non-technical reviewer. A correct test that produces an unreadable video fails this policy.

The goal: a human viewer should be able to follow each step and see the final result without pausing or rewinding. Tests that race through the UI in two seconds do not prove the spec to the stakeholder, even when they pass.

Apply these rules **only to the demo scenario**; other e2e tests stay fast and unrecorded:

- Scope recording to the demo test only. Prefer per-test configuration (Playwright `test.use({ video: 'on' })` inside the demo file while the global config stays `video: 'off'`, Cypress project split or `cy.task` gating, project-level `*.demo.spec.ts` matchers). Do not flip on global video recording.
- Use the framework's slow-mode knob so actions are visible: Playwright `use: { launchOptions: { slowMo: 300 } }` or per-test, Cypress via `cy.wait` discipline between actions, WebdriverIO `wdio.conf` `execArgv`, etc. Detect the framework first and apply its idiomatic mechanism. 250–500 ms per action is the target range.
- Prefer explicit assertion-based waits (`expect(locator).toBeVisible()`, `cy.contains(...).should('be.visible')`) over blind `sleep`/`wait(ms)`. Assertions double as pacing and as correctness checks, and they give the video a visible "something just happened" beat.
- After the final action, hold the end state visible for at least 1.5 seconds before the test ends, so the recorded frame captures the outcome rather than a teardown flash. A final visibility assertion followed by a short explicit wait is acceptable here — this is one of the few places where a fixed wait is justified.
- Record at a viewport large enough to show the relevant UI without cropping. Default to 1280×720 unless the project already standardises a larger size.
- One logical user action per step. Avoid chaining fills, clicks, and navigations into one line — each discrete action should be its own call so it appears as its own beat in the video.

**Run and artifacts**
- Detect the e2e run command and any required dev-server command from project conventions
- Start background services only when needed and wait for readiness
- Run the suite and verify that the expected artifacts are actually produced
- When a demo video is recorded, store it under `{config.paths.test_results}/{spec-id}/` (or document the framework-native artifact path in the final summary) and confirm it is present and playable before completing the spec
- Do not generate videos for non-demo e2e tests; if the framework default is `video: 'on'`, scope it down so only the demo scenario records
- Retry flaky or timeout-based failures once; if they fail again, report them clearly as non-transient

#### Progress reporting

After each wave, report briefly. See `./references/output-templates.md` for the "Wave Completion Report" template.

#### Before code review

After all implementation waves:
1. Run the project's unit and integration tests
2. Run e2e tests if this spec required or introduced them
3. If tests fail, determine whether the failure is new or pre-existing, fix local issues autonomously, and escalate only if an explicit blocker appears

### PHASE 3 - Code Review

**Main agent:** Cesare 🔍

- In `worker-backed preferred`, delegate review to a separate worker
- In `in-context fallback`, review in the current context
- Review only diffs or changed areas, using project conventions and the implementation plan as reference

**Review criteria:**
1. plan adherence
2. code quality
3. architecture adherence
4. security
5. test quality
6. mockup adherence when UI work exists
7. completeness vs. tasks and acceptance criteria

**Output format:** See `./eferences/output-templates.md` for the "Code Review Output" template.

### PHASE 4 - Fix & Re-Review Loop

If Cesare found critical issues:
1. Ugo and Mina fix them
2. Re-run the relevant tests
3. Re-review only the fix diffs
4. Repeat until no critical issues remain

If Cesare found only improvements:
1. Summarize them briefly.
2. Treat them as non-blocking by default.
3. Fix them only if the user explicitly asks for extra polishing, or if re-checking shows that one of them is actually critical.

If Cesare found no issues, or all critical issues are fixed, proceed to completion.

### Completion Gate

Proceed to Phase 5 only when all of the following are true:
- no `🔴 CRITICAL` findings remain open
- the full required final test suite passes
- the spec can be moved to `{config.workflow.statuses.review}` via `archetipo spec review`

`🟡 IMPROVEMENT` findings do not block completion by default.
Implementation is not complete until the spec status has been updated to `{config.workflow.statuses.review}`.
Do not end with the spec still in `{config.workflow.statuses.in_progress}`, and do not move it to `{config.workflow.statuses.done}` from this skill.

### PHASE 5 - Completion & Backlog Update

1. Run the full required test suite one final time. If it fails, return to the fix loop and do not transition the spec.
2. Pipe the completion summary markdown into `archetipo spec review {US-CODE}`. This single command transitions the spec to `{config.workflow.statuses.review}` AND posts the comment on the parent issue (or silently ignores it for connectors without comment support — never branch on connector type).
3. Confirm completion with a concise summary. See `references/output-templates.md` for the "Completion Summary" template. If non-blocking `🟡 IMPROVEMENT` items remain open, include them in the final report under an explicit optional improvements section.

## Edge Case Handling

- **Review loop exceeds 3 iterations:** summarize what remains and recommend whether to continue or re-evaluate
