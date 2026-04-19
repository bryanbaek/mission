"""Tests for repository-owned document CRUD helpers."""

import asyncio

from src.backend import repository


class StubDocumentConnection:
    def __init__(self) -> None:
        self.execute_calls: list[tuple[str, tuple[object, ...]]] = []
        self.fetchrow_calls: list[tuple[str, tuple[object, ...]]] = []
        self.fetchrow_result: dict | None = None

    async def execute(self, query: str, *args: object) -> object:
        self.execute_calls.append((query, args))
        return "DELETE 1"

    async def fetchrow(self, query: str, *args: object) -> dict | None:
        self.fetchrow_calls.append((query, args))
        return self.fetchrow_result


def test_write_document_uses_repository_upsert_query() -> None:
    conn = StubDocumentConnection()
    conn.fetchrow_result = {
        "id": "doc-123",
        "title": "Example Title",
        "content": "body",
    }

    document = asyncio.run(
        repository.write_document(conn, "doc-123", "Example Title", "body")
    )

    assert document == {"id": "doc-123", "title": "Example Title", "content": "body"}
    assert len(conn.fetchrow_calls) == 1
    query, args = conn.fetchrow_calls[0]
    assert "INSERT INTO documents" in query
    assert "ON CONFLICT (id)" in query
    assert "DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content" in query
    assert args == ("doc-123", "Example Title", "body")


def test_get_document_returns_normalized_row() -> None:
    conn = StubDocumentConnection()
    conn.fetchrow_result = {
        "id": "doc-123",
        "title": "Example Title",
        "content": "body",
    }

    document = asyncio.run(repository.get_document(conn, "doc-123"))

    assert document == {"id": "doc-123", "title": "Example Title", "content": "body"}
    assert len(conn.fetchrow_calls) == 1
    query, args = conn.fetchrow_calls[0]
    assert "SELECT id, title, content" in query
    assert "FROM documents" in query
    assert args == ("doc-123",)


def test_delete_document_uses_repository_delete_query() -> None:
    conn = StubDocumentConnection()

    asyncio.run(repository.delete_document(conn, "doc-123"))

    assert conn.execute_calls == [
        ("DELETE FROM documents WHERE id = $1", ("doc-123",))
    ]
