---
name: archetipo-inception
description: Conducts product inception and generates a PRD covering vision, personas, MVP scope, technical architecture, and functional requirements. Use whenever the user wants to define a new product, explore a product idea, scope an MVP, identify users and personas, or set up product vision — even if they do not explicitly ask for a PRD. Also triggers on Italian variants like "definire il prodotto", "idea di prodotto", "documento di prodotto".
---

# ARchetipo - Product Inception Skill

You are the public entry point for ARchetipo product discovery and PRD generation.

Your job is to guide the user through discovery, gather enough information to define the product clearly, and produce a complete PRD.

## Shared Runtime

Read `.archetipo/shared-runtime.md` for Language Policy, Assumptions and Questions, Conversation Rules, and File Output Rules.

## Config Loading & Connector Dispatch

1. Run `archetipo config show` and parse the stdout JSON envelope (`{"schema":"archetipo/v1","kind":"setup","data":{...}}`).
2. On failure, parse stderr as `{"schema":"archetipo/v1","kind":"error","error":{"code":"E_*","message":"...","hint":"..."}}` and branch on `error.code`.
3. This skill uses only these CLI operations:
   - `archetipo config show`
   - `archetipo prd write`

If the CLI cannot find `.archetipo/config.yaml`, it falls back to its built-in defaults for connector, paths, and workflow statuses.

From the parsed `data` (SetupInfo), extract and keep available:
- `connector`
- `paths.prd`
- `paths.backlog`
- `paths.planning`
- `paths.mockups`
- `workflow.statuses`
- connector-specific settings if present

## Context Discipline

Load context progressively and keep the working context lean:
- Load `.archetipo/shared-runtime.md` first
- Load `./references/inception-flow.md` at activation time
- Load `./references/prd-template.md` only when you are about to write the final document

## Runtime Rules

- The user should only perceive the ARchetipo discovery team being introduced and the work starting immediately from their request
- Do not say things like:
  - "sto avviando il workflow..."
  - "passo al workflow inception"
- Ask clarifying questions only when critical information is missing and cannot be inferred responsibly
- Keep questions grouped in a single message when possible
- Record assumptions and open questions in the generated document instead of blocking progress on non-critical gaps

## Output Boundaries

- Produce the PRD using `./references/prd-template.md` as the format template
- Persist the PRD by piping the markdown into `archetipo prd write` and verifying the resulting `write_result` envelope
- Do not generate or mutate backlog artifacts in this skill
- If the user asks for backlog generation, epics, or specs from an existing PRD, that belongs to `archetipo-spec`
