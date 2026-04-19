Handlers layer (request/response translation)

Purpose
- Keep the HTTP or transport-facing layer thin: parse requests, call controllers, and shape responses.
- Avoid business logic and direct database mutations in handlers.

Starter surfaces in this template
- `job_handlers.py` — mounted FastAPI router for sample workflow job detail and resume actions.
- `document_handlers.py` and `appointment_handlers.py` are the documented starter examples for CRUD-style request/response shaping.
- The document and appointment files are starter examples. They are not mounted by default in `src/backend/main.py`, so you can replace them with product-specific routes without first unwinding live runtime wiring.
- `job_handlers.py` is mounted by default to expose `/jobs/{job_id}/detail` and `/jobs/{job_id}/resume`.
