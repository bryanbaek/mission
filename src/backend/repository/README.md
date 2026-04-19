Repository layer (data access)

Purpose
- Keep persistence access in one place and out of controllers and handlers.
- Operate on open connections or transactions and return typed data structures.

Starter surfaces in this template
- `document_repository.py` — sample persistence helpers for the starter `documents` table.
- `appointment_repository.py` — sample persistence helpers for the starter `appointments` table.
- `src/backend/db/init-db.sql` — compose bootstrap schema for the sample CRUD tables plus durable workflow-state tables.
- The document and appointment tables are starter examples. Keep the repository boundary, but replace the sample tables and queries when you adapt the template to your own domain.

Notes
- There is no active Redis or Chroma repository layer in the checked-in runtime.
