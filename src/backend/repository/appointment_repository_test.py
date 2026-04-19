"""Tests for repository-owned appointment CRUD helpers."""

import asyncio
from datetime import datetime

from src.backend import repository


class StubAppointmentConnection:
    def __init__(self) -> None:
        self.execute_calls: list[tuple[str, tuple[object, ...]]] = []
        self.fetch_calls: list[tuple[str, tuple[object, ...]]] = []
        self.fetchrow_calls: list[tuple[str, tuple[object, ...]]] = []
        self.fetch_result: list[dict] = []
        self.fetchrow_result: dict | None = None

    async def execute(self, query: str, *args: object) -> object:
        self.execute_calls.append((query, args))
        return "DELETE 1"

    async def fetch(self, query: str, *args: object) -> list[dict]:
        self.fetch_calls.append((query, args))
        return self.fetch_result

    async def fetchrow(self, query: str, *args: object) -> dict | None:
        self.fetchrow_calls.append((query, args))
        return self.fetchrow_result


def test_has_conflict_uses_repository_overlap_query() -> None:
    conn = StubAppointmentConnection()
    conn.fetch_result = [{"exists": 1}]
    start_time = datetime(2026, 3, 21, 9, 0, 0)
    end_time = datetime(2026, 3, 21, 10, 0, 0)

    has_conflict = asyncio.run(repository.has_conflict(conn, start_time, end_time))

    assert has_conflict is True
    assert len(conn.fetch_calls) == 1
    query, args = conn.fetch_calls[0]
    assert "FROM appointments" in query
    assert "start_time < $2 AND end_time > $1" in query
    assert args == (start_time, end_time)


def test_has_conflict_excludes_current_appointment_when_requested() -> None:
    conn = StubAppointmentConnection()
    conn.fetch_result = []
    start_time = datetime(2026, 3, 21, 9, 0, 0)
    end_time = datetime(2026, 3, 21, 10, 0, 0)

    has_conflict = asyncio.run(
        repository.has_conflict(
            conn,
            start_time,
            end_time,
            exclude_appointment_id=7,
        )
    )

    assert has_conflict is False
    assert len(conn.fetch_calls) == 1
    query, args = conn.fetch_calls[0]
    assert "id != $3" in query
    assert args == (start_time, end_time, 7)


def test_create_appointment_returns_normalized_row() -> None:
    conn = StubAppointmentConnection()
    start_time = datetime(2026, 3, 21, 9, 0, 0)
    end_time = datetime(2026, 3, 21, 10, 0, 0)
    conn.fetchrow_result = {
        "id": 1,
        "title": "Weekly sync",
        "start_time": start_time,
        "end_time": end_time,
    }

    appointment = asyncio.run(
        repository.create_appointment(conn, "Weekly sync", start_time, end_time)
    )

    assert appointment == {
        "id": 1,
        "title": "Weekly sync",
        "start_time": start_time,
        "end_time": end_time,
    }
    assert len(conn.fetchrow_calls) == 1
    query, args = conn.fetchrow_calls[0]
    assert "INSERT INTO appointments" in query
    assert "RETURNING id, title, start_time, end_time" in query
    assert args == ("Weekly sync", start_time, end_time)


def test_get_appointments_returns_normalized_rows() -> None:
    conn = StubAppointmentConnection()
    first_start_time = datetime(2026, 3, 21, 9, 0, 0)
    first_end_time = datetime(2026, 3, 21, 10, 0, 0)
    second_start_time = datetime(2026, 3, 21, 11, 0, 0)
    second_end_time = datetime(2026, 3, 21, 12, 0, 0)
    conn.fetch_result = [
        {
            "id": 1,
            "title": "Weekly sync",
            "start_time": first_start_time,
            "end_time": first_end_time,
        },
        {
            "id": 2,
            "title": "Planning",
            "start_time": second_start_time,
            "end_time": second_end_time,
        },
    ]

    appointments = asyncio.run(repository.get_appointments(conn))

    assert appointments == [
        {
            "id": 1,
            "title": "Weekly sync",
            "start_time": first_start_time,
            "end_time": first_end_time,
        },
        {
            "id": 2,
            "title": "Planning",
            "start_time": second_start_time,
            "end_time": second_end_time,
        },
    ]
    assert len(conn.fetch_calls) == 1
    query, args = conn.fetch_calls[0]
    assert "SELECT id, title, start_time, end_time" in query
    assert "ORDER BY start_time ASC" in query
    assert args == ()


def test_get_appointment_returns_normalized_row() -> None:
    conn = StubAppointmentConnection()
    start_time = datetime(2026, 3, 21, 9, 0, 0)
    end_time = datetime(2026, 3, 21, 10, 0, 0)
    conn.fetchrow_result = {
        "id": 1,
        "title": "Weekly sync",
        "start_time": start_time,
        "end_time": end_time,
    }

    appointment = asyncio.run(repository.get_appointment(conn, 1))

    assert appointment == {
        "id": 1,
        "title": "Weekly sync",
        "start_time": start_time,
        "end_time": end_time,
    }
    assert len(conn.fetchrow_calls) == 1
    query, args = conn.fetchrow_calls[0]
    assert "SELECT id, title, start_time, end_time" in query
    assert "WHERE id = $1" in query
    assert args == (1,)


def test_update_appointment_returns_normalized_row() -> None:
    conn = StubAppointmentConnection()
    start_time = datetime(2026, 3, 21, 9, 0, 0)
    end_time = datetime(2026, 3, 21, 10, 0, 0)
    conn.fetchrow_result = {
        "id": 1,
        "title": "Weekly sync",
        "start_time": start_time,
        "end_time": end_time,
    }

    appointment = asyncio.run(
        repository.update_appointment(
            conn,
            1,
            "Weekly sync",
            start_time,
            end_time,
        )
    )

    assert appointment == {
        "id": 1,
        "title": "Weekly sync",
        "start_time": start_time,
        "end_time": end_time,
    }
    assert len(conn.fetchrow_calls) == 1
    query, args = conn.fetchrow_calls[0]
    assert "UPDATE appointments" in query
    assert "WHERE id = $4" in query
    assert args == ("Weekly sync", start_time, end_time, 1)


def test_delete_appointment_uses_repository_delete_query() -> None:
    conn = StubAppointmentConnection()

    asyncio.run(repository.delete_appointment(conn, 1))

    assert conn.execute_calls == [("DELETE FROM appointments WHERE id = $1", (1,))]
