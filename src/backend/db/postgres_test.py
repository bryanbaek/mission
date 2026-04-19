"""Tests for repository-owned Postgres bootstrap assets."""

import src.backend.db.postgres as postgres
from src.backend.db.postgres import (
    BOOTSTRAP_SQL_PATH,
    BOOTSTRAP_TABLES,
    read_bootstrap_sql,
)


def test_read_bootstrap_sql_loads_expected_tables() -> None:
    sql = read_bootstrap_sql()

    assert BOOTSTRAP_SQL_PATH.exists()
    for table_name in BOOTSTRAP_TABLES:
        assert f"CREATE TABLE IF NOT EXISTS {table_name}" in sql


def test_postgres_module_only_exports_bootstrap_helpers() -> None:
    assert postgres.__all__ == [
        "BOOTSTRAP_SQL_PATH",
        "BOOTSTRAP_TABLES",
        "read_bootstrap_sql",
    ]
