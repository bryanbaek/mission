## Skills
Repo-local skills live under `skills/`. Do not create `SKILLS.md`; each skill gets its own folder with a `SKILL.md`.

### Available skills
- berry-pr-draft-workflow: Submit or inspect local PR draft jobs through `localhost:8000`, poll `/jobs/{id}` until a terminal state, and summarize validation, PR, and failure metadata. Use when the user asks to run or debug the PR drafting API flow. (file: /Users/baek/Desktop/berry/skills/berry-pr-draft-workflow/SKILL.md)
- berry-real-provider-e2e: Run the real-provider E2E workflow against the local stack and collect app/db failure context. Use when the user asks to run or debug `scripts/verify_real_providers_e2e.sh` or related real-provider checks. (file: /Users/baek/Desktop/berry/skills/berry-real-provider-e2e/SKILL.md)

### How to use skills
- If the user names a skill or the task clearly matches one, open its `SKILL.md` and follow it.
- Resolve relative paths from the skill directory.
- Use bundled scripts when present instead of retyping long command sequences.
- Load extra reference files only when the active task needs them.

## Repo conventions
### Python regex policy
- Prefer `regex` for regex-based parsing, matching, and substitution in Python.
- Use plain string methods when regex is unnecessary.
- Use `re` only when stdlib-specific behavior or compatibility is explicitly required; leave a short comment when you do.

### Canonical commands
- Prefer repo entrypoints in `Justfile` over hand-built command sequences.
- For backend changes, use narrower checks while iterating when they reduce cycle time, then run `just lint` and `just test` before handoff.
- Use `just format` only when you intend to rewrite files.

### Local stack commands
- Prefer `just compose-up`, `just compose-down`, `just compose-rebuild`, `just compose-status`, `just compose-shell`, and related `compose-*` targets over raw `docker-compose`.
- Fall back to raw `docker-compose` only when the needed behavior has no `Justfile` target, such as collecting non-following tail logs.

### Shell behavior
- Never rely on shell aliases such as `g`; use full commands like `git`.
- Prefer repo scripts or skills over repeated ad hoc `curl` loops for local workflow automation.

### Localhost workflow automation
- For `/pr-drafts` creation and `/jobs/{id}` polling, prefer `skills/berry-pr-draft-workflow/scripts/run_pr_draft_workflow.sh` or the matching skill over hand-written polling commands.

### File size and modularity
- Keep non-generated source files under 2000 lines of code. If a file grows beyond 2000 LOC, refactor it into smaller, focused modules before adding more behavior.
- Treat files approaching 2000 LOC as a design warning, not a target. Split earlier when single responsibility is weak, cognitive load is rising, or the change surface area is spreading across unrelated concerns.
- Refactor around clear seams so each module has one primary reason to change. Avoid mixing high-level orchestration with low-level implementation details in the same file.
- In Python, separate business logic from I/O. Put outbound API and service integration logic under `src/backend/gateway/`, and put database access under `src/backend/repository/postgres/` instead of embedding that logic in large controller or service files.
- Common large-file smells include mixing business logic with DB/API/filesystem I/O, multiple unrelated classes or functions in one file, oversized `utils.py` or `helpers.py` modules, procedural scripts that kept growing, and code that mixes multiple abstraction levels.

## Where To Edit X
- HTTP routes and request/response shaping: edit `src/backend/engine/handlers/` for generic workflow APIs or `src/backend/sdlc/handlers/` for PM, Slack, and PR-draft product APIs. Router mounting lives in `src/backend/main.py`.
- Core workflow engine behavior: edit `src/backend/engine/controller/` for jobs, tasks, dispatch, retries, escalation, and observability.
- Background scheduling and task execution: edit `src/backend/engine/services/` and `src/backend/engine/worker/`.
- PM intake, requirements drafting, Linear, PR drafting, Slack product behavior: edit `src/backend/sdlc/controller/` and `src/backend/sdlc/tools/`.
- Third-party integrations: edit `src/backend/gateway/`.
- Persistence and schema-backed queries: edit `src/backend/repository/postgres/`. Shared DTOs and enums live in `src/backend/models/`.
- Generic tool execution or new tools: edit `src/backend/engine/tools/` and `src/backend/engine/tools/implementations/`.
- Frontend dashboard behavior: edit `src/frontend/src/pages/`, `src/frontend/src/components/`, `src/frontend/src/api.ts`, and `src/frontend/src/types.ts`.
- Scripts, smoke tests, and manual verification flows: edit `scripts/` and `docs/`.
- Codex repo guidance and automation: edit `AGENTS.md` and `skills/`.
- Prefer canonical namespaces first. `src/backend/handlers/`, `src/backend/services/`, and `src/backend/worker/` are compatibility wrappers, not the primary place for new behavior.
