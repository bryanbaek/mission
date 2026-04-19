"""Tests for sample job workflow orchestration."""

import asyncio

import pytest

from src.backend.controller.job_controller import (
    InvalidJobIdError,
    JobController,
    JobNotFoundError,
    JobNotResumableError,
)
from src.backend.models.job import JobDetailDTO


class StubJobGateway:
    def __init__(self) -> None:
        self.jobs = {
            "job-paused-123": JobDetailDTO(
                job_id="job-paused-123",
                workflow_type="pm_tool_workflow",
                current_state="awaiting_requirements",
                status="paused",
                state_version=3,
                resume_count=0,
                can_resume=True,
                last_error="Waiting for product requirements",
            ),
            "job-completed-789": JobDetailDTO(
                job_id="job-completed-789",
                workflow_type="pm_tool_workflow",
                current_state="published_summary",
                status="completed",
                state_version=8,
                resume_count=0,
                can_resume=False,
            ),
            "job-running-456": JobDetailDTO(
                job_id="job-running-456",
                workflow_type="pm_tool_workflow",
                current_state="drafting_pr",
                status="in_progress",
                state_version=5,
                resume_count=1,
                can_resume=True,
            ),
        }
        self.requested_ids: list[str] = []
        self.saved_jobs: list[JobDetailDTO] = []

    async def get_job(self, job_id: str) -> JobDetailDTO | None:
        self.requested_ids.append(job_id)
        job = self.jobs.get(job_id)
        if job is None:
            return None
        return job.model_copy(deep=True)

    async def save_job(self, job: JobDetailDTO) -> JobDetailDTO:
        self.saved_jobs.append(job.model_copy(deep=True))
        self.jobs[job.job_id] = job.model_copy(deep=True)
        return job.model_copy(deep=True)


def test_get_job_detail_normalizes_job_id() -> None:
    gateway = StubJobGateway()
    controller = JobController(gateway)

    job = asyncio.run(controller.get_job_detail("  job-paused-123  "))

    assert gateway.requested_ids == ["job-paused-123"]
    assert job == gateway.jobs["job-paused-123"]


def test_get_job_detail_rejects_blank_job_ids() -> None:
    controller = JobController(StubJobGateway())

    with pytest.raises(InvalidJobIdError, match="job_id must not be empty"):
        asyncio.run(controller.get_job_detail("   "))


def test_resume_job_updates_resumable_job_state() -> None:
    gateway = StubJobGateway()
    controller = JobController(gateway)

    resumed_job = asyncio.run(controller.resume_job("job-paused-123"))

    assert resumed_job.current_state == "drafting_pr"
    assert resumed_job.status == "in_progress"
    assert resumed_job.state_version == 4
    assert resumed_job.resume_count == 1
    assert resumed_job.can_resume is True
    assert resumed_job.last_error is None
    assert gateway.saved_jobs == [resumed_job]


def test_resume_job_allows_in_progress_jobs() -> None:
    gateway = StubJobGateway()
    controller = JobController(gateway)

    resumed_job = asyncio.run(controller.resume_job("job-running-456"))

    assert resumed_job.status == "in_progress"
    assert resumed_job.state_version == 6
    assert resumed_job.resume_count == 2
    assert resumed_job.can_resume is True


def test_resume_job_rejects_unknown_jobs() -> None:
    controller = JobController(StubJobGateway())

    with pytest.raises(JobNotFoundError, match="job not found"):
        asyncio.run(controller.resume_job("job-missing-000"))


def test_resume_job_rejects_non_resumable_jobs() -> None:
    controller = JobController(StubJobGateway())

    with pytest.raises(
        JobNotResumableError,
        match="job is not resumable from status 'completed'",
    ):
        asyncio.run(controller.resume_job("job-completed-789"))
