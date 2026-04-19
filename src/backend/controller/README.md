Controller layer (workflow orchestration)

Purpose
- Normalize input and enforce workflow rules before touching I/O.
- Keep controllers reusable and deterministic by delegating external work to collaborators.

Starter surfaces in this template
- These starter examples center on `job_controller.py`, plus the documented `document_controller.py` and `appointment_controller.py` sample flows described in the root README and plan.
- `job_controller.py` shows identifier normalization, typed detail retrieval, and explicit resume rules for a small workflow-oriented controller.
- Keep the controller boundary, but replace the sample document, appointment, and job rules when you adapt the template to a real product.
