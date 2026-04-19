"""FastAPI routes for job detail and resume workflow actions."""

from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Request, status

from src.backend.controller.job_controller import (
    InvalidJobIdError,
    JobController,
    JobNotFoundError,
    JobNotResumableError,
    build_job_controller,
)
from src.backend.models.job import JobDetailDTO, ResumeJobResponseDTO

router = APIRouter(prefix="/jobs", tags=["jobs"])
job_router = router


def get_job_controller(request: Request) -> JobController:
    """Return the configured job controller, creating the sample one if needed."""
    controller = getattr(request.app.state, "job_controller", None)
    if controller is None:
        controller = build_job_controller()
        request.app.state.job_controller = controller
    return controller


@router.get(
    "/{job_id}/detail",
    response_model=JobDetailDTO,
)
async def get_job_detail(
    job_id: str,
    controller: Annotated[JobController, Depends(get_job_controller)],
) -> JobDetailDTO:
    """Return workflow-oriented detail for a single job."""
    try:
        return await controller.get_job_detail(job_id)
    except InvalidJobIdError as exc:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(exc),
        ) from exc
    except JobNotFoundError as exc:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=str(exc),
        ) from exc


@router.post(
    "/{job_id}/resume",
    response_model=ResumeJobResponseDTO,
)
async def resume_job(
    job_id: str,
    controller: Annotated[JobController, Depends(get_job_controller)],
) -> ResumeJobResponseDTO:
    """Resume a paused or in-progress workflow job."""
    try:
        job = await controller.resume_job(job_id)
    except InvalidJobIdError as exc:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(exc),
        ) from exc
    except JobNotFoundError as exc:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=str(exc),
        ) from exc
    except JobNotResumableError as exc:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=str(exc),
        ) from exc
    return ResumeJobResponseDTO(job=job)


__all__ = ["get_job_controller", "job_router", "router"]
