# Architecture

One-page reference for the current stack decisions. Rationale is short by design.

## Product shape

Multi-tenant SaaS. Each tenant = a privacy-sensitive SMB running their own on-prem database. Cloud control plane + Docker edge agent on tenant infra + cloud LLM that sees schema only, never raw rows.

## Three planes

| Plane | Runs where | Role |
|---|---|---|
| Control plane | DigitalOcean App Platform (ours) | Web UI, auth, tenant config, semantic layer, agent orchestration, LLM router |
| Edge agent | Tenant's own infra (Docker) | Read-only MySQL gateway, SQL execution, local audit log |
| LLM | Anthropic / OpenAI / Together / Mistral / Cerebras / DeepSeek / xAI / Fireworks (cloud) | Receives schema + NL question; returns SQL and summaries. Never sees raw rows in Tier 1. |

## Stack

| Concern | Choice | Why |
|---|---|---|
| Backend language | Go (`1.25`) | Team strength; single static binary; same language across control plane and edge agent |
| Web framework | [go-chi](https://github.com/go-chi/chi) | Stdlib-compatible, composable middleware, no magic |
| RPC | [Connect-RPC](https://connectrpc.com/) | gRPC-compatible protobuf, runs over plain HTTP/1+JSON and HTTP/2, no Envoy proxy, Go + TS codegen |
| Control-plane DB | Postgres 16 (DigitalOcean Managed Databases) | Boring, reliable, same-vendor consolidation |
| Postgres driver | [pgx v5 + pgxpool](https://github.com/jackc/pgx) | Fastest, most featureful Go Postgres driver |
| Auth | [Clerk](https://clerk.com) | SaaS-only; JWT verification via middleware; removes hand-rolled auth risk |
| Frontend | Vite + React + TypeScript + Tailwind + shadcn/ui | Plain SPA, no SSR/edge concerns; backend-eng-friendly mental model |
| Frontend hosting | DigitalOcean App Platform Static Sites | Same vendor as backend |
| LLM providers | Anthropic, OpenAI, Together, Mistral, Cerebras, DeepSeek, xAI, and Fireworks behind a `gateway/llm.Provider` interface | Switching per tenant = config change, not code change |
| Edge-agent transport | Connect-RPC server streams over HTTPS (SSE-shaped) for commands, unary POST for results; mTLS | Tenant is behind NAT (outbound only). Simpler ops than WebSocket; aligns with Connect stack. |
| SQL safety | Pure-Go AST parser (`pingcap/tidb/parser`) + read-only MySQL user + read replica + `LIMIT` injection + statement timeout | Defense in depth — no single layer is trusted |
| Container registry | DigitalOcean Container Registry | Same vendor |
| Object storage (later) | DigitalOcean Spaces (S3-compatible) | Same vendor, standard API |

## Layered structure (in Go)

Mirrors the conventions from the original template. Enforced per package.

```
cmd/
  control-plane/        main.go — wires config → db → repos → controllers → handlers → router
  edge-agent/           main.go — agent binary
internal/
  controlplane/
    config/             env loading, typed config
    db/                 Postgres pool lifecycle
    model/              domain structs (Tenant, User, SemanticLayer, Report, …)
    repository/         Postgres access — one repo per aggregate, returns model.*
    gateway/            outbound integrations
      llm/              LLM Provider interface + Anthropic/OpenAI-compatible/Mistral implementations
    controller/         workflow orchestration; deterministic; no HTTP or SQL
    handler/            thin HTTP / Connect-RPC handlers — parse request, call controller, shape response
  edgeagent/            same layered shape
proto/                  Connect-RPC .proto schemas
web/                    Vite + React SPA
```

**Layer rules:**
- Handler → Controller → (Gateway | Repository). Never skip downward or reverse.
- Models are plain structs. No methods that do I/O.
- Repositories never leak `pgx` types upward.
- Gateways never absorb business logic.
- Tenant identity flows via `context.Context` — never hardcoded, never as a string literal in code.

## Data flow (text-to-SQL, Tier 1)

```
user (browser)
  → control-plane: NL question + tenant ctx
  → LLM router: schema + semantic layer + NL → SQL
  → SQL AST validator: reject if not SELECT/WITH, inject LIMIT
  → edge agent (tenant infra): execute SQL on read replica
  → results back to control plane
  → LLM: summarize results in Korean
  → user
```

What never leaves tenant network: raw tables, full row dumps. What does leave: schema, semantic layer, NL question, and (Tier 1) query-result rows for summarization. Tier 2 moves summarization local.

## Hosting (ours)

Single vendor: DigitalOcean. One bill, one dashboard.

- App Platform: control-plane container + web static site
- Managed Postgres: control-plane DB
- Container Registry: edge-agent Docker images
- Spaces (later): generated PDFs, exports

External dependencies we accept: Clerk (auth SaaS), and whichever enabled LLM APIs the deployment uses.
