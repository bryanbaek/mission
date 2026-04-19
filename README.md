# FastAPI Template

This repository is a reusable FastAPI template with a small set of starter examples. The checked-in document and appointment flows are intentionally sample code, not required product logic. A lightweight Bun + Vite + TypeScript page in `src/frontend` is included for browser-facing bootstrap work.

For a repository-aligned example of adapting this template into a personal finance tracker, see `FINANCE_APP_IMPLEMENTATION_PLAN.md`.

## Before Using This As A Template

Use the starter code as a verification target before you replace it with your own domain:

1. Review the starter surfaces you plan to keep versus replace.

- Keep the layer boundaries in `src/backend/controller`, `src/backend/gateway`, `src/backend/repository`, and `src/backend/handlers`.
- Treat the sample document and appointment flows in those directories as starter examples.
- Treat `src/frontend` as an optional demo UI that proves the backend/frontend wiring, not as required long-term product code.

2. Copy the example environment file and decide which integrations still belong in your project.

```bash
cp env.example .env
```

- `docker-compose.yaml` and the local Postgres bootstrap rely on `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `DATABASE_URL`, `BACKEND_PORT`, `FRONTEND_PORT`, and `VITE_BACKEND_URL`.
- If you keep the optional OpenAI starter integration in `src/backend/gateway/llm_gateway.py`, also set `OPENAI_API_KEY` and `OPENAI_MODEL`.
- If you use the Plaid gateway in `src/backend/gateway/plaid`, set `PLAID_CLIENT_ID`, `PLAID_SECRET`, and `PLAID_ENV`. Optionally override the Plaid API version header with `PLAID_API_VERSION`.

3. Verify the template before you customize it.

```bash
uv sync
cd src/frontend
bun install
cd ../..
docker compose up --build
just lint
just test
```

- `uv sync` installs the backend toolchain declared in `pyproject.toml`.
- `bun install`, `bun run dev`, and `bun run build` come from `src/frontend/package.json`.
- Run `docker compose up --build` from the repo root after copying `env.example` to `.env`.
- The `Template Readiness` GitHub Actions workflow runs the same `uv sync`, `just lint`, and `just test` entrypoints on pull requests to `main` and via manual dispatch.

4. Replace starter examples only after the repository is green.

- Swap the sample document and appointment modules for your own domain behavior while keeping the layer seams.
- Update `src/frontend/src/main.ts` after your own backend routes and URLs are in place.
- Keep `src/backend/main.py` minimal beyond the mounted sample job workflow routes until you decide which product routes should actually be mounted by default.
- The minimal bootstrap contract only mounts `/` and `/health` for smoke checks, even though the checked-in template also includes the sample job router until you replace it.

## Starter Layout

The template includes a lightweight frontend starter in `src/frontend` plus starter backend layers in `src/backend`:

- `src/frontend`: Bun + Vite + TypeScript demo page. Optional starter UI.
- `src/backend/controller`: sample document, appointment, and job orchestration. Keep the controller boundary, replace the sample rules.
- `src/backend/gateway`: outbound integrations. `llm_gateway.py` is an optional starter integration, and `plaid/` adds a minimal Plaid wrapper for Link, Auth, Accounts, Item, and Transactions Sync flows.
- `src/backend/repository`: Postgres read/write helpers for the sample document and appointment flows.
- `src/backend/handlers`: thin request/response helpers around the sample flows plus a mounted sample job workflow router.
- `src/backend/db/postgres.py`: bootstrap helpers for checked-in database assets.
- `src/backend/db/init-db.sql`: sample tables plus durable workflow-state tables used by local Postgres bootstrap.

The default FastAPI app in `src/backend/main.py` mounts `/`, `/health`, `/jobs/{job_id}/detail`, and `/jobs/{job_id}/resume`. The document and appointment CRUD modules are starter examples that are unit tested in place, but they are not wired into live routes until you choose how to expose them.

## Starter Examples

The sample backend includes three small starter flows:

- Job workflow example: `src/backend/controller/job_controller.py` and `src/backend/handlers/job_handlers.py`

`write_document` keeps create-or-update semantics so a single starter flow covers both initial creation and replacement. The appointment sample shows payload normalization, time-range validation, and conflict checks.

## Frontend Demo

The frontend starter lives in `src/frontend`. It is intentionally small: a single Bun + Vite + TypeScript page that links to the FastAPI root and health endpoints. Replace it after the template bootstrap checks pass and your own product routes exist.

## Run

### Backend

Install backend dependencies from the repo root with:

```bash
uv sync
```

Start the FastAPI app with:

```bash
uv run uvicorn src.backend.main:app --reload
```

The included FastAPI app starts from `src/backend/main.py` and exposes `/`, `/health`, `/jobs/{job_id}/detail`, and `/jobs/{job_id}/resume`.

### Frontend

The frontend starter uses Bun. Install dependencies from the repo root with:

```bash
cd src/frontend
bun install
```

Start the Vite development server with:

```bash
cd src/frontend
bun run dev
```

Then open `http://localhost:5173` in a browser. The demo page links to the FastAPI starter using `VITE_BACKEND_URL`, which defaults to `http://localhost:8000` for local development.

Create a production build with:

```bash
cd src/frontend
bun run build
```

The built assets are written to `src/frontend/dist/`.

### Docker Compose

For the full local stack, copy `env.example` to `.env`, adjust any ports or credentials you need, and then start the services:

```bash
cp env.example .env
docker compose up --build
```

By default, the compose stack exposes:

- FastAPI on `http://localhost:8000`
- Bun/Vite frontend on `http://localhost:5173`
- Postgres on `localhost:5432`

`docker-compose.yaml` reads its local database and app settings from the variables copied from `env.example` into `.env`, including `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `BACKEND_PORT`, `FRONTEND_PORT`, and `VITE_BACKEND_URL`.

The frontend service runs the starter app in `src/frontend`, the backend service runs `src.backend.main:app`, and the Postgres service mounts the bootstrap SQL from `src/backend/db/init-db.sql`.
If you change the published backend URL, update `VITE_BACKEND_URL` so the frontend starter still points at the FastAPI service.

The Postgres data directory lives in the named `postgres-data` volume.
That means `docker compose stop`, `docker compose start`, and `docker compose down` preserve starter and workflow state.
Run `docker compose down -v` only when you want to remove the named volume and force Postgres to initialize a fresh database on the next `docker compose up`.

## Postgres Bootstrap

The checked-in bootstrap script lives at `src/backend/db/init-db.sql`.
It creates the sample `documents` and `appointments` tables expected by the starter repository modules, plus the durable workflow tables `workflow_state_store` and `workflow_state_transitions`.
The Python helper in `src/backend/db/postgres.py` only reads that bootstrap asset; it does not own sample-specific query logic.

When you start the compose stack, `docker-compose.yaml` mounts that SQL file into the official Postgres entrypoint directory at `/docker-entrypoint-initdb.d/init-db.sql`.
The Postgres image applies files from that directory only when it initializes a fresh data directory, so the named `postgres-data` volume preserves both schema and data across container recreation.
If you need the bootstrap script to run again after changing the schema, remove the volume with `docker compose down -v` before starting the stack again.

Apply the schema to an existing local Postgres database with:

```bash
psql "$DATABASE_URL" -f src/backend/db/init-db.sql
```

## Verify

After syncing dependencies, run the repository verification commands from the repo root:

```bash
just lint
just test
```

`just lint` runs `uv run ruff check .` followed by `uv run ty check`.
`just test` runs `uv run pytest`, which includes colocated `*_test.py` modules under `src/` plus any tests under `tests/`.

The `Template Readiness` GitHub Actions workflow runs `uv sync`, `just lint`, and `just test` on pull requests to `main` and via manual dispatch. That workflow also validates that this README, `Justfile`, `pyproject.toml`, `docker-compose.yaml`, `env.example`, and the backend layer README files stay aligned.

## Development

Use these files first while adapting the template:

- `src/backend/main.py`
- `src/backend/main_test.py`
- `src/backend/controller/README.md`
- `src/backend/controller/job_controller.py`
- `src/backend/gateway/README.md`
- `src/backend/gateway/llm_gateway.py`
- `src/backend/gateway/plaid/client.py`
- `src/backend/handlers/README.md`
- `src/backend/handlers/job_handlers.py`
- `src/backend/repository/README.md`
- `src/backend/repository/appointment_repository.py`
- `src/backend/db/postgres.py`
- `src/backend/db/init-db.sql`
- `src/frontend/package.json`
- `src/frontend/src/main.ts`
- `src/frontend/src/style.css`

## Notes

- The sample flows are deliberately small and are meant to be replaced.
- The repository keeps Docker, GitHub Actions, FastAPI app wiring, and starter tests so new projects can bootstrap from a working baseline.
