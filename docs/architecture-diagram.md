# Architecture Diagram

Generated from the current codebase and architecture docs:

- `ARCHITECTURE.md`
- `README.md`
- `cmd/control-plane/main.go`
- `cmd/edge-agent/main.go`
- `internal/controlplane/controller/query*.go`
- `internal/edgeagent/controller/agent.go`
- `web/src/App.tsx`

## System Context

```mermaid
flowchart LR
    subgraph User["User Environment"]
        Browser["Browser<br/>React + Vite SPA"]
        Clerk["Clerk<br/>JWT auth"]
    end

    subgraph Cloud["Cloud Control Plane"]
        Static["Frontend hosting<br/>static site or bundled assets"]

        subgraph CP["Control Plane (Go)"]
            API["chi router + Connect-RPC handlers"]
            Auth["Auth middleware<br/>Clerk verifier or dev fake verifier"]
            Controllers["Controllers<br/>tenant, query, semantic, onboarding"]
            Sessions["AgentSessionManager<br/>long-lived agent sessions"]
            Repo["Repositories"]
            LLMRouter["LLM router<br/>provider selection + failover"]
        end

        PG["Postgres 16<br/>tenant config, schema versions,<br/>semantic layer, onboarding,<br/>query runs, feedback"]
    end

    subgraph Providers["LLM Providers"]
        OpenAI["OpenAI"]
        Anthropic["Anthropic"]
        Others["Mistral / Together / Cerebras /<br/>DeepSeek / xAI / Fireworks"]
    end

    subgraph Tenant["Tenant Infrastructure"]
        Agent["Edge Agent (Go)<br/>Connect client + command stream"]
        MySQL["MySQL read replica / read-only user"]
        Audit["Local audit log<br/>JSONL query events"]
    end

    Browser -->|"loads app"| Static
    Browser -->|"fetches /app-config.json"| API
    Browser -->|"sign-in"| Clerk
    Browser -->|"JWT-authenticated Connect-RPC"| API

    API --> Auth --> Controllers
    Controllers --> Repo --> PG
    Controllers --> LLMRouter
    Controllers --> Sessions

    LLMRouter --> OpenAI
    LLMRouter --> Anthropic
    LLMRouter --> Others

    Sessions -. "commands over HTTP/2 stream" .-> Agent
    Agent -. "results, heartbeats, schema snapshots" .-> Sessions
    Agent -->|"validated read-only SQL"| MySQL
    Agent -->|"append audit event"| Audit
```

## Text-to-SQL Request Flow

```mermaid
sequenceDiagram
    participant U as User
    participant B as Browser SPA
    participant CP as Control Plane
    participant PG as Postgres
    participant LLM as LLM Router / Provider
    participant AG as Edge Agent
    participant DB as Tenant MySQL
    participant AL as Local Audit Log

    U->>B: Ask a natural-language question
    B->>CP: QueryService.AskQuestion(JWT, tenantId, question)
    CP->>PG: Load membership, schema, semantic layer, examples
    PG-->>CP: Tenant context

    CP->>LLM: Generate SQL from schema + semantic layer + question
    LLM-->>CP: SQL candidate
    Note over CP: sqlguard validates the AST,<br/>blocks unsafe constructs,<br/>and injects LIMIT when needed

    CP->>AG: ExecuteQuery(rewritten SQL)
    AG->>DB: Run read-only SQL
    DB-->>AG: Columns + rows
    AG->>AL: Append JSONL audit event
    AG-->>CP: Query result + timing + DB metadata

    CP->>LLM: Summarize results in Korean
    LLM-->>CP: Summary
    CP->>PG: Persist query run, attempts, warnings
    CP-->>B: SQL, rows, summary, warnings

    Note over LLM,DB: Tier 1 boundary:<br/>LLM receives schema for SQL generation<br/>and result rows for summarization,<br/>but never connects directly to the tenant DB
```

## Layered Code Shape

```mermaid
flowchart TD
    Handler["handler/<br/>HTTP + Connect adapters"]
    Controller["controller/<br/>workflow orchestration"]
    Repository["repository/<br/>Postgres persistence"]
    Gateway["gateway/<br/>LLM and external integrations"]
    Model["model/<br/>plain domain structs"]
    External["External systems<br/>Postgres, LLMs, edge agent"]

    Handler --> Controller
    Controller --> Repository
    Controller --> Gateway
    Repository --> Model
    Gateway --> External
```
