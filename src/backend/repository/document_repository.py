"""Repository layer for the template document sample."""

from typing import Protocol


class DocumentConnection(Protocol):
    """Minimal async database interface required by repository functions."""

    async def execute(self, query: str, *args: object) -> object: ...

    async def fetchrow(self, query: str, *args: object) -> dict | None: ...


def _row_to_document(row: dict | None) -> dict | None:
    """Convert a database row to a document dictionary payload."""
    if row is None:
        return None
    return {"id": row["id"], "title": row["title"], "content": row["content"]}


async def write_document(
    conn: DocumentConnection, document_id: str, title: str, content: str
) -> dict:
    """Create or update and return a document."""
    row = await conn.fetchrow(
        """
        INSERT INTO documents (id, title, content)
        VALUES ($1, $2, $3)
        ON CONFLICT (id)
        DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content
        RETURNING id, title, content
        """,
        document_id,
        title,
        content,
    )
    document = _row_to_document(row)
    if document is None:
        return {"id": document_id, "title": title, "content": content}
    return document


async def get_document(conn: DocumentConnection, document_id: str) -> dict | None:
    """Fetch a single document by id."""
    row = await conn.fetchrow(
        """
        SELECT id, title, content
        FROM documents
        WHERE id = $1
        """,
        document_id,
    )
    return _row_to_document(row)


async def delete_document(conn: DocumentConnection, document_id: str) -> None:
    """Delete a document by id."""
    await conn.execute("DELETE FROM documents WHERE id = $1", document_id)


__all__ = [
    "DocumentConnection",
    "delete_document",
    "get_document",
    "write_document",
]
