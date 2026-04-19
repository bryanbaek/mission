"""Environment-backed runtime settings for backend integrations."""

from __future__ import annotations

import os
from collections.abc import Mapping

from pydantic import BaseModel, ConfigDict, Field

DEFAULT_OPENAI_TIMEOUT_S = 30.0
DEFAULT_OPENAI_MAX_RETRIES = 2


def _read_optional_env(
    environ: Mapping[str, str],
    key: str,
    *,
    default: str | float | int | None = None,
) -> str | float | int | None:
    value = environ.get(key)
    if value is None:
        return default

    normalized = value.strip()
    if not normalized:
        return default
    return normalized


class OpenAISettings(BaseModel):
    """Configuration required by the OpenAI-backed LLM gateway."""

    model_config = ConfigDict(frozen=True)

    api_key: str | None = None
    model: str | None = None
    base_url: str | None = None
    timeout_s: float = Field(default=DEFAULT_OPENAI_TIMEOUT_S, gt=0)
    max_retries: int = Field(default=DEFAULT_OPENAI_MAX_RETRIES, ge=0)

    @classmethod
    def from_env(cls, environ: Mapping[str, str] | None = None) -> "OpenAISettings":
        """Build OpenAI settings from environment variables."""
        source = os.environ if environ is None else environ
        return cls.model_validate(
            {
                "api_key": _read_optional_env(source, "OPENAI_API_KEY"),
                "model": _read_optional_env(source, "OPENAI_MODEL"),
                "base_url": _read_optional_env(source, "OPENAI_BASE_URL"),
                "timeout_s": _read_optional_env(
                    source,
                    "OPENAI_TIMEOUT_S",
                    default=DEFAULT_OPENAI_TIMEOUT_S,
                ),
                "max_retries": _read_optional_env(
                    source,
                    "OPENAI_MAX_RETRIES",
                    default=DEFAULT_OPENAI_MAX_RETRIES,
                ),
            }
        )


def get_openai_settings(environ: Mapping[str, str] | None = None) -> OpenAISettings:
    """Return the repository's OpenAI settings object."""
    return OpenAISettings.from_env(environ)


__all__ = [
    "DEFAULT_OPENAI_MAX_RETRIES",
    "DEFAULT_OPENAI_TIMEOUT_S",
    "OpenAISettings",
    "get_openai_settings",
]
