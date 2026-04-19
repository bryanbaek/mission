# Finance App Implementation Plan

This document maps the current FastAPI template to a personal finance tracking app inspired by Mint and Monarch. It follows the existing README guidance: verify the template first, keep the documented layer boundaries, and then replace the starter examples with finance-specific workflows.

## Product Goal

Build an initial finance MVP that helps a single user or household:

- create and manage financial accounts
- ingest transactions through manual entry and CSV imports
- categorize income, expenses, and transfers
- set monthly budgets by category
- review balances, cash flow, and spending summaries from a dashboard

## Working Assumptions

- Start with a single-user or single-household MVP to keep the first schema and API surface small.
- Treat manual account setup and CSV imports as the first transaction ingestion path.
- Keep the existing FastAPI, Postgres, Docker Compose, and Bun/Vite setup from the template rather than introducing new infrastructure in the first pass.
- Keep the sample code only long enough to verify the template and replace it incrementally after finance workflows are available.

## MVP Scope

### In Scope

- Accounts: manual setup for checking, savings, credit card, cash, loan, and investment accounts
- Transactions: manual creation and edit flows plus CSV import for historical transaction ingestion
- Categories: default finance categories plus user-editable categories for income, expenses, and transfers
- Budgets: monthly category budgets with actual-versus-budget reporting
- Dashboard reporting: total balances, recent transactions, monthly income versus spending, and category spend summaries

### Later Enhancements

- Direct bank or card aggregation integrations such as Plaid, MX, or Finicity
- Advanced analytics such as forecasting, net worth projections, subscription detection, or anomaly detection
- Notifications, reminders, or recurring transaction automation
- AI categorization, AI insights, or LLM-backed financial summaries
- Multi-user collaboration, auth/roles, or household sharing beyond the initial owner model

## Keep Versus Replace By Template Surface

| Surface | Keep | Replace or retire | Finance-domain target |
| --- | --- | --- | --- |
| `src/backend/controller` | Keep the controller boundary and colocated test style. | Replace the sample orchestration in `job_controller.py` once real finance workflows exist, or repurpose its orchestration pattern for long-running imports or syncs. | Add controllers for accounts, transactions, categories, budgets, dashboard summaries, and optional import jobs. |
| `src/backend/gateway` | Keep the gateway boundary for outbound services and provider-specific configuration. | Treat `llm_gateway.py` as optional starter code. It is not required for the finance MVP and should remain unused or be removed in later cleanup unless a real finance feature needs it. | Financial data providers, including the checked-in Plaid wrapper in `src/backend/gateway/plaid`, plus exchange-rate lookups, export services, or notification providers belong here. |
| `src/backend/repository` | Keep the repository boundary and Postgres-first access pattern. | Replace `document_repository.py`, `appointment_repository.py`, `schema.sql`, and the sample `documents` and `appointments` schema in `src/backend/db/init-db.sql`. | Add finance repositories for accounts, transactions, categories, budgets, and dashboard/reporting queries. |
| `src/backend/handlers` | Keep thin FastAPI request/response translation and minimal router mounting in `src/backend/main.py`. | Replace or retire the sample `job_handlers.py` routes once finance APIs exist, unless the same pattern is reused for async import jobs. | Add finance API routes for account management, transaction workflows, budgets, reports, and dashboard summaries. |
| `src/frontend` | Keep Bun, Vite, TypeScript, the existing dev/build scripts in `src/frontend/package.json`, and the `VITE_BACKEND_URL` wiring. | Replace the single demo page in `src/frontend/src/main.ts` and `src/frontend/src/style.css` after finance routes are ready. | Build the finance dashboard, account list, transaction review screens, budget views, and settings pages here. |

## Finance Domain Mapping

### Backend Layers

| Layer | Finance responsibility |
| --- | --- |
| `src/backend/controller` | Business rules for manual account setup, transaction normalization, categorization rules, budget updates, and dashboard summary assembly |
| `src/backend/gateway` | Optional external provider adapters such as future bank aggregation, exchange-rate lookup, CSV storage, notifications, or exports |
| `src/backend/repository` | Postgres persistence for finance entities plus reporting queries and import deduplication helpers |
| `src/backend/handlers` | FastAPI request parsing, response shaping, error translation, and route mounting for finance workflows |
| `src/frontend` | Optional browser UI for dashboard views, account management, transaction review, budgeting, and settings |

### Recommended Initial Data Model Surfaces

| Surface | Purpose | Suggested first fields |
| --- | --- | --- |
| `accounts` | Store manual financial accounts and current balance metadata. | `id`, `name`, `account_type`, `institution_name`, `currency`, `current_balance`, `is_manual`, `last_imported_at`, `created_at`, `updated_at` |
| `transactions` | Store normalized financial entries for each account. | `id`, `account_id`, `external_id`, `posted_at`, `amount`, `description`, `merchant_name`, `category_id`, `notes`, `source`, `status`, `created_at`, `updated_at` |
| `categories` | Define income, expense, and transfer categories. | `id`, `name`, `parent_category_id`, `kind`, `is_system`, `is_active`, `created_at`, `updated_at` |
| `monthly_budgets` | Track budget targets per category and month. | `id`, `month`, `category_id`, `target_amount`, `notes`, `created_at`, `updated_at` |
| reporting queries or views | Power dashboard summaries without a separate analytics stack in the first release. | monthly cash flow, spending by category, account balance rollups, budget-versus-actual summaries |
| import job tracking | Track CSV imports or future sync jobs if they become asynchronous. | reuse the existing workflow-state tables or add a finance-specific import tracking table later |

If shared access is expected soon after MVP, add an `owner_id` or `household_id` consistently across finance tables from the first migration. If not, keep the first version single-user and avoid premature auth scope.

## Recommended API And Workflow Surfaces

| Workflow | Initial handler/API surface | Controller focus | Repository or gateway notes |
| --- | --- | --- | --- |
| Manual account setup | `GET /accounts`, `POST /accounts`, `PATCH /accounts/{account_id}` | validate account type, currency, opening balance, and display ordering | persists to `accounts`; no gateway needed for MVP |
| Transaction ingestion | `GET /transactions`, `POST /transactions`, `PATCH /transactions/{transaction_id}` | normalize signed amounts, dates, notes, and category assignment | persists to `transactions`; support account/date/category filters |
| CSV transaction import | `POST /transactions/imports` | parse files, deduplicate rows, surface import errors, and apply categories | use repository dedupe helpers; reuse the sample job workflow only if imports need background execution |
| Categorization | `GET /categories`, `POST /categories`, `PATCH /categories/{category_id}` | enforce category kind rules and support uncategorized review queues | persists to `categories`; AI or provider gateways stay out of MVP |
| Budgets | `GET /budgets`, `PUT /budgets/{month}` | enforce one budget target per category and month, calculate budget status | persists to `monthly_budgets`; query actual spend from `transactions` |
| Dashboard reporting | `GET /dashboard/summary`, `GET /reports/spending-by-category`, `GET /reports/monthly-cashflow` | assemble balance, cash-flow, and budget summary DTOs | start with direct SQL aggregation queries rather than a separate analytics service |

## Frontend MVP Plan

Replace the starter `src/frontend` demo incrementally instead of rebuilding the frontend all at once.

Recommended first pages and components:

- dashboard page with balance cards, spending charts, monthly cash-flow summary, and recent transactions
- accounts page with account list, balances, and add or edit account forms
- transactions page with filters, import action, category editing, and search
- budgets page with monthly budget grid, category progress, and over-budget states
- settings page for category management and basic app preferences such as default currency
- shared components for summary cards, transaction tables, category selectors, empty states, and error banners

Keep the existing toolchain in `src/frontend/package.json`, use `VITE_BACKEND_URL` for backend wiring, and preserve the Bun/Vite development flow documented in `README.md`.

## Local Development And Deployment Considerations

- Follow the README bootstrap path first: copy `env.example` to `.env`, run `uv sync`, install frontend dependencies from `src/frontend`, and start the stack with `docker compose up --build`.
- Keep the current local topology from `docker-compose.yaml`: FastAPI backend on `BACKEND_PORT`, Bun/Vite frontend on `FRONTEND_PORT`, and Postgres on `POSTGRES_PORT`.
- For early deployment targets, keep the same container boundary as local development: the existing FastAPI app image, Postgres persistence, and the current frontend build/runtime path before introducing new infrastructure.
- Replace the sample `documents` and `appointments` schema in `src/backend/db/init-db.sql` only after the template verification step is complete and finance repositories are ready to consume the new tables.
- Continue using `just lint` and `just test` as the documented repo entrypoints while finance code is added.
- Add finance tests alongside the new controllers, repositories, and handlers using the existing `*_test.py` pattern under `src/`.
- Treat `.github/workflows/python-app.yml` and `.github/workflows/pylint.yml` as the current CI baseline. As finance functionality expands, keep those workflows aligned with the repo’s chosen test and lint commands in a follow-up ticket if the toolchain changes.
- `env.example` currently includes template-era AI and datastore variables. The finance MVP should rely only on the settings it actually uses, and unused template variables can be removed in later cleanup once the sample integrations are retired.

## Incremental Delivery Milestones

1. Template verification and inventory

Confirm the existing README flow works locally, keep `/` and `/health` as baseline checks, and treat the sample job routes as temporary proof that FastAPI routing and controller wiring are functional.

2. Finance schema and backend foundation

Define the first finance entities, replace sample repositories and tables with finance equivalents, add finance DTOs and controllers, and keep the repository and handler boundaries intact.

3. Accounts and transaction ingestion

Implement manual account setup, manual transaction CRUD, and CSV transaction import. Reuse the job-workflow pattern only if imports need resumable or long-running processing.

4. Categories, budgets, and reports

Add category management, monthly budgets, budget-versus-actual queries, and summary endpoints for dashboard reporting.

5. Frontend MVP and template cleanup

Replace the demo frontend with finance screens, connect it to the finance APIs, then remove obsolete sample document and appointment code plus any no-longer-used template configuration.

## Explicit Non-MVP Work

The following work should stay out of the first finance MVP:

- direct account aggregation and credentialed bank connections
- investment lot tracking, tax reporting, or complex liability amortization
- proactive notifications, bill reminders, and recurring payment automation
- AI-generated financial advice, natural-language transaction search, or LLM-backed categorization
- advanced analytics pipelines, data warehouses, or separate reporting services
