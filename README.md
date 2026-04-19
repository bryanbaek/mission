# Mission

Multi-tenant AI-over-your-database product. See [ARCHITECTURE.md](./ARCHITECTURE.md) for the stack and [/Users/baek/.claude/plans/a-korean-client-reached-velvet-mountain.md](../.claude/plans/a-korean-client-reached-velvet-mountain.md) for the strategic plan and week-by-week action items.

## Prerequisites

- Go 1.23+
- Node 20+
- Docker + Docker Compose
- [just](https://github.com/casey/just) (optional but recommended)

## Quick start (docker compose)

```bash
cp env.example .env
docker compose up --build
```

- Frontend: http://localhost:5173
- Backend `/healthz`: http://localhost:8080/healthz
- Postgres: `postgres://mission:mission@localhost:5432/mission`

## Quick start (local processes)

```bash
# Terminal 1 — Postgres
docker compose up postgres

# Terminal 2 — Go backend
export DATABASE_URL=postgres://mission:mission@localhost:5432/mission?sslmode=disable
just run-control-plane        # or: go run ./cmd/control-plane

# Terminal 3 — web
just web-install
just web-dev                   # http://localhost:5173
```

The Vite dev server proxies `/healthz` and `/api` to `localhost:8080`.

## Layout

```
cmd/                    executable entry points (control-plane, edge-agent)
internal/controlplane/  control-plane packages (handler/controller/gateway/repository/model/db/config)
internal/edgeagent/     edge-agent packages (filled in Week 2+)
proto/                  Connect-RPC schemas (codegen wired in 1.2)
web/                    Vite + React + Tailwind SPA
Dockerfile.control-plane, Dockerfile.edge-agent
docker-compose.yaml     local dev stack (postgres + control-plane + web)
Justfile                common commands
```

## Deploying the control plane to DigitalOcean App Platform

Week 1.1 target: push the Docker image and run it on App Platform.

```bash
# Build and push to DO Container Registry (replace <registry>)
doctl registry login
docker build -f Dockerfile.control-plane -t registry.digitalocean.com/<registry>/control-plane:latest .
docker push registry.digitalocean.com/<registry>/control-plane:latest

# Create a Managed Postgres cluster in the DO dashboard, then set the
# control-plane app's env vars from its connection string.
```

Minimum env vars on App Platform: `ENV=production`, `HTTP_PORT=8080`, `DATABASE_URL=<managed-postgres-url>`, `LOG_LEVEL=info`.

## Verify

```bash
curl http://localhost:8080/healthz
# {"status":"ok","database":"ok"}
```
