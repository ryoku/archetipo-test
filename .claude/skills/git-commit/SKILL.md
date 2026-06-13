---
name: git-commit
description: Commit the files currently in the git staging area, generating a commit message that accurately reflects the staged changes and follows the Conventional Commits standard. Use this skill whenever the user wants to commit staged changes — phrases like "commit", "commit the staged files", "fai il commit", "committa", "create a commit", "wrap this up in a commit", or any request to record staged work in git history. Use it even when the user does not explicitly mention "Conventional Commits" — this skill is the default way to produce well-formed commits.
---

# Git Commit (Conventional Commits)

Commit the changes that are **already staged** in git, with a message that an
experienced maintainer would write after reading the diff: accurate, scoped, and
formatted according to the [Conventional Commits](https://www.conventionalcommits.org)
specification.

The whole point of this skill is that the message must be _derived from the actual
staged diff_, not guessed from the conversation or from filenames alone. Read what
changed, understand the intent, then describe it.

## Core workflow

Follow these steps in order. Do not skip the inspection step — the message quality
depends entirely on it.

### 1. Confirm there is something staged

Run:

```bash
git status --short
git diff --cached --stat
```

- If `git diff --cached --stat` is empty, **nothing is staged**. Stop and tell the
  user there are no staged changes, and ask whether they want to stage something
  (e.g. `git add <files>`) first. Do **not** run `git add` automatically and do
  **not** use `git commit -a` — this skill commits only what the user has
  deliberately staged.
- If there are both staged and unstaged changes to the same files, mention this
  briefly so the user knows the unstaged part will not be included.

### 2. Read the staged diff

```bash
git diff --cached
```

Read the full diff, not just the file list. Identify:

- **What** changed: new feature, bug fix, refactor, docs, tests, config, etc.
- **Where** it changed: which module/package/area — this becomes the optional scope.
- **Why** it likely changed, as far as the diff reveals it.
- Whether there is a **breaking change** (removed/renamed public API, changed
  function signature, changed config format, altered behavior callers rely on).

For large diffs, also look at recent history to match the project's existing style:

```bash
git log --oneline -15
```

This tells you the prevailing scope names, the language the team writes messages in,
and whether they use bodies/footers.

### 3. Choose the type

Pick the single type that best matches the _primary_ purpose of the change:

| Type       | Use when the change…                                                       |
| ---------- | -------------------------------------------------------------------------- |
| `feat`     | adds a new feature or user-facing capability                               |
| `fix`      | fixes a bug                                                                |
| `docs`     | touches only documentation (README, comments-as-docs, docs site)           |
| `style`    | only affects formatting/whitespace/semicolons, no logic change             |
| `refactor` | restructures code without changing behavior or fixing a bug                |
| `perf`     | improves performance                                                       |
| `test`     | adds or fixes tests only                                                   |
| `build`    | affects build system, packaging, or dependencies (e.g. npm, cargo, poetry) |
| `ci`       | changes CI configuration or scripts (GitHub Actions, GitLab CI, etc.)      |
| `chore`    | maintenance that doesn't fit above (tooling, config, housekeeping)         |
| `revert`   | reverts a previous commit                                                  |

If a single commit genuinely spans several types, prefer the type of the most
significant change, and consider telling the user the staged set might be better
split into multiple commits.

### 4. Choose an optional scope

The scope is a short noun in parentheses naming the section of the codebase
affected, e.g. `feat(auth):`, `fix(parser):`, `docs(readme):`.

- Derive it from the directory/module the diff touches, and reuse names already
  present in `git log` when possible.
- Omit the scope if the change is broad or no single scope fits. Don't invent a
  noisy or overly specific scope.

### 5. Write the description (the subject line)

Format: `<type>(<optional scope>)<optional !>: <description>`

Rules for the description:

- Imperative mood, present tense: "add", "fix", "remove" — not "added" / "adds".
- Lowercase first letter, **no** trailing period.
- Concise but specific — aim for ≤ 50 characters, hard limit ~72. Keep the whole
  subject line under 72 characters.
- Describe the change, not the file: `fix(api): handle empty pagination cursor`,
  not `fix(api): update handler.js`.

### 6. Add a body and footers only when they add value

Separate the subject from the body with one blank line.

- **Body** (optional): explain the _why_ and any non-obvious context, wrapped at
  ~72 columns. Skip it for small, self-evident changes — don't pad.
- **Breaking changes**: if the diff breaks compatibility, signal it **both** ways
  recommended by the spec: put a `!` before the colon in the subject _and_ add a
  footer starting exactly with `BREAKING CHANGE: ` describing what broke and the
  migration path.
- **Issue references**: if the user mentioned an issue/ticket, add a footer like
  `Refs: #123` or `Closes: #123`. Don't fabricate issue numbers.

### 7. Commit

Use a multi-line commit so the body and footers are preserved. Prefer passing the
message via repeated `-m` flags or a heredoc — never embed unescaped quotes:

```bash
git commit -m "feat(auth): add token refresh on 401" \
           -m "Retries the original request once after silently refreshing the
access token, so users are not bounced to the login screen on expiry." \
           -m "Closes: #214"
```

After committing, run `git log -1 --stat` and show the user the resulting commit so
they can confirm it looks right.

## Language

Write the commit message in English unless explicitly asked. The type
keywords (`feat`, `fix`, …) and the literal `BREAKING CHANGE:` token always stay in
English regardless of the description language, since they are part of the spec.

## Hard constraints

- Never stage files the user didn't stage; never use `git commit -a`. Commit the
  staging area exactly as the user prepared it.
- Never invent changes that aren't in the diff just to make the message richer.
- Don't add promotional or attribution lines to the commit unless the user asks.
- If the staged diff and the user's stated intent clearly conflict, point it out
  before committing rather than silently trusting one over the other.

## Examples

**Example 1 — simple fix**
Staged diff: a null check added before dereferencing `user.profile` in `profile.ts`.
Message:

```
fix(profile): guard against missing user profile
```

**Example 2 — feature with context**
Staged diff: new CSV export endpoint plus a route and a serializer.
Message:

```
feat(reports): add CSV export endpoint

Streams rows so large reports don't buffer fully in memory. The
serializer reuses the existing column config from the JSON exporter.
```

**Example 3 — breaking change**
Staged diff: `createClient(url)` changed to `createClient({ url })`.
Message:

```
refactor(client)!: take an options object in createClient

BREAKING CHANGE: createClient now accepts a single options object
instead of positional arguments. Replace createClient(url) with
createClient({ url }).
```
