"""Repository layer for the sample appointment workflow."""

from datetime import datetime
from typing import Protocol


class AppointmentConnection(Protocol):
    """Minimal async database interface required by appointment repositories."""

    async def execute(self, query: str, *args: object) -> object: ...

    async def fetch(self, query: str, *args: object) -> list[dict]: ...

    async def fetchrow(self, query: str, *args: object) -> dict | None: ...


def _row_to_appointment(row: dict | None) -> dict | None:
    if row is None:
        return None
    return {
        "id": row["id"],
        "title": row["title"],
        "start_time": row["start_time"],
        "end_time": row["end_time"],
    }


async def has_conflict(
    conn: AppointmentConnection,
    start_time: datetime,
    end_time: datetime,
    exclude_appointment_id: int | None = None,
) -> bool:
    """Return whether a proposed appointment overlaps an existing booking."""
    if exclude_appointment_id is None:
        rows = await conn.fetch(
            """
            SELECT 1
            FROM appointments
            WHERE start_time < $2 AND end_time > $1
            LIMIT 1
            """,
            start_time,
            end_time,
        )
        return bool(rows)

    rows = await conn.fetch(
        """
        SELECT 1
        FROM appointments
        WHERE start_time < $2 AND end_time > $1 AND id != $3
        LIMIT 1
        """,
        start_time,
        end_time,
        exclude_appointment_id,
    )
    return bool(rows)


async def create_appointment(
    conn: AppointmentConnection, title: str, start_time: datetime, end_time: datetime
) -> dict:
    """Create and return an appointment."""
    row = await conn.fetchrow(
        """
        INSERT INTO appointments (title, start_time, end_time)
        VALUES ($1, $2, $3)
        RETURNING id, title, start_time, end_time
        """,
        title,
        start_time,
        end_time,
    )
    appointment = _row_to_appointment(row)
    if appointment is None:
        return {
            "title": title,
            "start_time": start_time,
            "end_time": end_time,
        }
    return appointment


async def get_appointment(
    conn: AppointmentConnection, appointment_id: int
) -> dict | None:
    """Fetch a single appointment by id."""
    row = await conn.fetchrow(
        """
        SELECT id, title, start_time, end_time
        FROM appointments
        WHERE id = $1
        """,
        appointment_id,
    )
    return _row_to_appointment(row)


async def get_appointments(conn: AppointmentConnection) -> list[dict]:
    """Fetch all appointments ordered by start time."""
    rows = await conn.fetch(
        """
        SELECT id, title, start_time, end_time
        FROM appointments
        ORDER BY start_time ASC
        """
    )
    appointments: list[dict] = []
    for row in rows:
        appointment = _row_to_appointment(row)
        if appointment is not None:
            appointments.append(appointment)
    return appointments


async def update_appointment(
    conn: AppointmentConnection,
    appointment_id: int,
    title: str,
    start_time: datetime,
    end_time: datetime,
) -> dict | None:
    """Update and return an appointment."""
    row = await conn.fetchrow(
        """
        UPDATE appointments
        SET title = $1, start_time = $2, end_time = $3
        WHERE id = $4
        RETURNING id, title, start_time, end_time
        """,
        title,
        start_time,
        end_time,
        appointment_id,
    )
    return _row_to_appointment(row)


async def delete_appointment(conn: AppointmentConnection, appointment_id: int) -> None:
    """Delete an appointment by id."""
    await conn.execute("DELETE FROM appointments WHERE id = $1", appointment_id)


__all__ = [
    "AppointmentConnection",
    "create_appointment",
    "delete_appointment",
    "get_appointment",
    "get_appointments",
    "has_conflict",
    "update_appointment",
]
