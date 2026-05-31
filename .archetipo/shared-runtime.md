# ARchetipo Shared Runtime

This file contains runtime rules shared by all ARchetipo skills.
Load this file once at activation time, before loading any flow reference.

## CLI Runtime Contract

ARchetipo skills use `archetipo` as the only backend for PRD, backlog, plan, task, and workflow-status operations.

Common rules:

- Run `archetipo config show` at the start of every skill that needs project metadata or configured paths.
- Parse stdout as a JSON success envelope:

```json
{"schema":"archetipo/v1","kind":"<kind>","data":{...}}
```

- Parse stderr as a JSON error envelope:

```json
{"schema":"archetipo/v1","kind":"error","error":{"code":"E_*","message":"...","hint":"..."}}
```

- Branch on `error.code`, never on `error.message`.
- Treat exit codes as stable:
  - `0`: success
  - `1`: generic error
  - `2`: invalid input
  - `3`: connector/backend failure
  - `4`: missing precondition
- When `.archetipo/config.yaml` is absent, the CLI applies its built-in defaults for connector, paths, and workflow statuses.
- Command-specific invocation forms, payloads, and semantics belong in each skill that uses them. Do not infer CLI operations from documentation files.

## Language Policy

Detect the output language from the strongest available source, in priority order:
1. Language of the backlog (if a backlog exists and is readable)
2. Language of the PRD (if no backlog is available)
3. Language of the user's current conversation

Apply the detected language to all user-facing output: messages, document section headers, error messages, and opening announcements.

### Template Rendering Rule

Templates and example text in skill files are **structural guides written in English**. When generating the final artifact, render every static element in the detected language. This includes:

- Document titles and section headings (e.g. "Elevator Pitch", "Vision", "User Personas")
- Table headers (e.g. "Phase | Action | Thought | Emotion | Opportunity")
- Bold inline labels (e.g. "**Author:**", "**Role:**", "**Goals:**", "**Pain Points:**")
- Connective phrases and sentence scaffolding (e.g. "For **X**, who has the problem of **Y**, **Z** is a **C** that..." → translate the connectives "For", "who has the problem of", "is a", "that", "Unlike", "our product")
- Enumerations, captions, footers, and any hard-coded prose around placeholders
- Agent role captions (e.g. "Proposed by:")

Rules:
1. Keep every `{{PLACEHOLDER}}` token **unchanged** — do not translate placeholder names.
2. Keep code blocks, file paths, CLI commands, and identifiers unchanged.
3. Keep technical terms that have no natural translation (e.g. "MVP", "ADR", "CI/CD", "ORM") unchanged unless the target language has a standard equivalent already used in the existing artifact.
4. Keep consistency with any existing artifact language (PRD → backlog → specs must all use the same language).
5. If the detected language is English, render the template as-is.

The final output must read as a single coherent document in the detected language — never a mix of English scaffolding and localized content.


## Assumptions and Questions

Ask the user only when all these conditions are true:
1. The missing information is critical to generate a correct output
2. The information cannot be reasonably inferred from the rest of the context
3. Proceeding would likely create a materially wrong result

If questions are needed:
- ask at most 3
- group them in one message
- allow the user to skip them

For non-critical gaps:
- infer a reasonable assumption
- continue
- record the assumption or open question in the final artifact

## Conversation Rules

- Each agent speaks in character
- Never mention internal mode names, workflow names, or routing decisions in the conversation

## Agent Persona

When an agent speaks, always render the speaker as `icon + name`, for example:

```text
💎 Andrea: [content]

🧭 Costanza: [content]
```

This rule applies to any skill that defines named agents with personas.

## File Output Rules

- Use the configured output path whenever present
- Create parent directories if they do not exist
- Overwrite the target generated artifact for the current run unless the active flow explicitly says otherwise
- When a connector overrides write-output behavior, follow that connector for I/O and keep the domain logic unchanged
