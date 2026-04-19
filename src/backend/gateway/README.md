Gateway layer (outbound integrations)

Purpose
- Encapsulate calls to third-party services so controllers stay deterministic and testable.
- Provide small, typed facades that can be mocked in unit tests.

Starter surfaces in this template
- `llm_gateway.py` — optional OpenAI-backed starter integration for structured outputs.
- `plaid/` — Plaid gateway wrapper for Link token creation, public token exchange, Auth, Accounts, Item, and Transactions Sync calls.
- This repository does not require the gateway layer to be active for local bootstrap. Keep individual integrations only if your project still needs them.

OpenAI gateway configuration
- `OPENAI_API_KEY` (required when using the OpenAI-backed gateway)
- `OPENAI_MODEL` (required when using the OpenAI-backed gateway)
- `OPENAI_BASE_URL` (optional; for compatible endpoints)
- `OPENAI_TIMEOUT_S` (default `30`)
- `OPENAI_MAX_RETRIES` (default `2`)

Plaid gateway configuration
- `PLAID_CLIENT_ID` (required when using `PlaidGateway` without an injected client)
- `PLAID_SECRET` (required when using `PlaidGateway` without an injected client)
- `PLAID_ENV` (default `sandbox`; supported values are `sandbox` and `production`)
- `PLAID_API_VERSION` (optional; default `2020-09-14`)

Guidelines
- Keep APIs minimal and explicit; no global clients.
- Return structured results or raise domain-specific exceptions; avoid leaking provider-specific objects upward.
- Make timeouts and configuration injectable; prefer dependency injection in controllers and services.
- Ensure gateways stay side-effect focused only and do not absorb business logic.
