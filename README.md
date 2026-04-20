# Mission

Multi-tenant AI-over-your-database product. See [ARCHITECTURE.md](./ARCHITECTURE.md) for the stack and [DIGITALOCEAN.md](./DIGITALOCEAN.md) for a production deployment guide.

## Prerequisites

- Go 1.25+
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
- Debug agent UI: http://localhost:5173
- Postgres: `postgres://mission:mission@localhost:5432/mission`
- MySQL 8: `mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app` and `mission_rw:mission_rw@tcp(127.0.0.1:3306)/mission_app`

## Quick start (local processes)

```bash
# Terminal 1 — Postgres + MySQL
docker compose up postgres mysql

# Terminal 2 — Go backend
export DATABASE_URL=postgres://mission:mission@localhost:5432/mission?sslmode=disable
just run-control-plane        # or: go run ./cmd/control-plane

# Terminal 3 — web
just web-install
just web-dev                   # http://localhost:5173

# Terminal 4 — edge agent (after issuing a tenant token)
mkdir -p /tmp/mission
printf '%s\n' 'mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app' > /tmp/mission/mysql.dsn
export CONTROL_PLANE_URL=http://localhost:8080
export TENANT_TOKEN=<issued-agent-token>
export MYSQL_DSN_FILE=/tmp/mission/mysql.dsn
just run-edge-agent
```

The Vite dev server proxies `/healthz` and `/api` to `localhost:8080`.

## MySQL DSN file

The edge agent does not read MySQL credentials from the control plane. It
expects a local file whose contents are only the DSN.

Local file example:

```text
mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app
```

Docker mount pattern:

```bash
docker run \
  -e CONTROL_PLANE_URL=http://host.docker.internal:8080 \
  -e TENANT_TOKEN=<issued-agent-token> \
  -e MYSQL_DSN_FILE=/etc/mission/mysql.dsn \
  -v /absolute/path/mysql.dsn:/etc/mission/mysql.dsn:ro \
  mission/edge-agent:latest
```

Local docker-compose MySQL users:

```text
Read-only:  mission_ro:mission_ro@tcp(127.0.0.1:3306)/mission_app
Read-write: mission_rw:mission_rw@tcp(127.0.0.1:3306)/mission_app
```

## Layout

```
cmd/                    executable entry points (control-plane, edge-agent)
internal/controlplane/  control-plane packages (handler/controller/gateway/repository/model/db/config)
internal/edgeagent/     edge-agent packages
proto/                  Connect-RPC schemas and generated clients
web/                    Vite + React + Tailwind SPA
Dockerfile.control-plane, Dockerfile.edge-agent
docker-compose.yaml     local dev stack (postgres + mysql + control-plane + web)
Justfile                common commands
```

## Deployment

For production deployment on DigitalOcean App Platform, use [DIGITALOCEAN.md](./DIGITALOCEAN.md). It covers the control-plane service, web static site, managed Postgres, ingress rules, and the edge-agent image registry flow.

## Verify

```bash
curl http://localhost:8080/healthz
# {"status":"ok","database":"ok"}

go test ./...
```
