"""Postgres bootstrap helpers for repository-owned database assets.

Workflow-specific CRUD queries live under ``src.backend.repository``.
"""

from pathlib import Path

BOOTSTRAP_SQL_PATH = Path(__file__).with_name("init-db.sql")
BOOTSTRAP_TABLES = (
    "documents",
    "appointments",
    "workflow_state_store",
    "workflow_state_transitions",
)


def read_bootstrap_sql() -> str:
    """Return the checked-in SQL used to initialize local Postgres state."""
    return BOOTSTRAP_SQL_PATH.read_text(encoding="utf-8")


__all__ = ["BOOTSTRAP_SQL_PATH", "BOOTSTRAP_TABLES", "read_bootstrap_sql"]
