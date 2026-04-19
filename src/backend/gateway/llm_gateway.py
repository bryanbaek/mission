"""LLM gateway abstractions and OpenAI structured-output implementation."""

from __future__ import annotations

import json
from abc import ABC, abstractmethod
from collections.abc import Iterable, Mapping
from typing import Any, TypeVar

from pydantic import BaseModel, ValidationError

from src.backend.settings import OpenAISettings, get_openai_settings

AsyncOpenAI: Any | None
try:
    from openai import AsyncOpenAI as _AsyncOpenAI
except ImportError:  # pragma: no cover - depends on local environment setup
    AsyncOpenAI = None
else:
    AsyncOpenAI = _AsyncOpenAI

ResponseModelT = TypeVar("ResponseModelT", bound=BaseModel)


class LLMGatewayError(RuntimeError):
    """Base error for gateway-level LLM failures."""


class LLMGatewayConfigurationError(LLMGatewayError):
    """Raised when gateway configuration is incomplete or invalid."""


class LLMOutputMissingError(LLMGatewayError):
    """Raised when the provider response does not include text output."""


class LLMOutputJSONError(LLMGatewayError):
    """Raised when provider output is not valid JSON."""


class LLMOutputValidationError(LLMGatewayError):
    """Raised when provider JSON does not satisfy the response schema."""


class LLMGateway(ABC):
    """Abstract contract for structured LLM completions."""

    @abstractmethod
    async def structured_completion(
        self,
        *,
        system_prompt: str,
        user_prompt: str,
        response_model: type[ResponseModelT],
    ) -> ResponseModelT:
        """Return a validated structured response for the supplied prompts."""


def _read_field(value: Any, key: str, default: Any = None) -> Any:
    if isinstance(value, Mapping):
        return value.get(key, default)
    return getattr(value, key, default)


def _iter_items(value: Any) -> Iterable[Any]:
    if value is None or isinstance(value, (str, bytes)):
        return ()
    if isinstance(value, Iterable):
        return value
    return ()


def _normalize_json_schema(value: Any) -> Any:
    if isinstance(value, dict):
        normalized = {
            key: _normalize_json_schema(item)
            for key, item in value.items()
        }
        if normalized.get("type") == "object" and "additionalProperties" not in normalized:
            normalized["additionalProperties"] = False
        return normalized
    if isinstance(value, list):
        return [_normalize_json_schema(item) for item in value]
    return value


class OpenAILLMGateway(LLMGateway):
    """OpenAI-backed LLM gateway for strict structured completions."""

    def __init__(
        self,
        *,
        settings: OpenAISettings | None = None,
        client: Any | None = None,
    ) -> None:
        resolved_settings = get_openai_settings() if settings is None else settings
        if not resolved_settings.model:
            raise LLMGatewayConfigurationError(
                "OPENAI_MODEL is required to initialize OpenAILLMGateway."
            )
        if client is None and not resolved_settings.api_key:
            raise LLMGatewayConfigurationError(
                "OPENAI_API_KEY is required to initialize OpenAILLMGateway "
                "when no client is injected."
            )
        if client is None and AsyncOpenAI is None:
            raise LLMGatewayConfigurationError(
                "The openai package must be installed when no client is injected."
            )

        self._settings = resolved_settings
        self.last_response_id: str | None = None
        self._client = client if client is not None else self._build_client()

    async def structured_completion(
        self,
        *,
        system_prompt: str,
        user_prompt: str,
        response_model: type[ResponseModelT],
    ) -> ResponseModelT:
        if not issubclass(response_model, BaseModel):
            raise TypeError("response_model must be a Pydantic BaseModel type")

        response = await self._client.responses.create(
            model=self._settings.model,
            input=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            text={"format": self._build_response_format(response_model)},
        )
        self.last_response_id = _read_field(response, "id")

        output_text = self._extract_output_text(response)
        payload = self._parse_json_payload(output_text, response_model)
        return self._validate_payload(payload, response_model)

    def _build_client(self) -> Any:
        if AsyncOpenAI is None:  # pragma: no cover - guarded in __init__
            raise LLMGatewayConfigurationError(
                "The openai package must be installed when no client is injected."
            )
        client_kwargs: dict[str, Any] = {
            "api_key": self._settings.api_key,
            "timeout": self._settings.timeout_s,
            "max_retries": self._settings.max_retries,
        }
        if self._settings.base_url:
            client_kwargs["base_url"] = self._settings.base_url
        return AsyncOpenAI(**client_kwargs)

    def _build_response_format(
        self,
        response_model: type[BaseModel],
    ) -> dict[str, Any]:
        return {
            "type": "json_schema",
            "name": self._schema_name(response_model),
            "schema": self._build_response_schema(response_model),
            "strict": True,
        }

    def _build_response_schema(
        self,
        response_model: type[BaseModel],
    ) -> dict[str, Any]:
        """Build the JSON schema sent to OpenAI for structured outputs."""
        return _normalize_json_schema(response_model.model_json_schema())

    def _schema_name(self, response_model: type[BaseModel]) -> str:
        schema_name = response_model.__name__ or "structured_response"
        sanitized = "".join(
            character if character.isalnum() or character in {"_", "-"} else "_"
            for character in schema_name
        ).strip("_")
        if not sanitized:
            return "structured_response"
        if sanitized[0].isdigit():
            return f"schema_{sanitized}"
        return sanitized

    def _extract_output_text(self, response: Any) -> str:
        output_text = _read_field(response, "output_text")
        if isinstance(output_text, str) and output_text.strip():
            return output_text

        text_fragments: list[str] = []
        for output_item in _iter_items(_read_field(response, "output")):
            for content_item in _iter_items(_read_field(output_item, "content")):
                if _read_field(content_item, "type") not in {"output_text", "text"}:
                    continue
                text_value = _read_field(content_item, "text")
                if isinstance(text_value, str) and text_value:
                    text_fragments.append(text_value)

        aggregated_text = "".join(text_fragments)
        if aggregated_text.strip():
            return aggregated_text

        raise LLMOutputMissingError(
            "OpenAI response did not include any output text to parse."
        )

    def _parse_json_payload(
        self,
        output_text: str,
        response_model: type[BaseModel],
    ) -> Any:
        try:
            return json.loads(output_text)
        except json.JSONDecodeError as exc:
            raise LLMOutputJSONError(
                f"OpenAI response for {response_model.__name__} was not valid JSON: "
                f"{exc.msg} (line {exc.lineno}, column {exc.colno})."
            ) from exc

    def _validate_payload(
        self,
        payload: Any,
        response_model: type[ResponseModelT],
    ) -> ResponseModelT:
        try:
            return response_model.model_validate(payload)
        except ValidationError as exc:
            raise LLMOutputValidationError(
                "OpenAI response JSON did not match the expected schema for "
                f"{response_model.__name__}: {exc}"
            ) from exc


__all__ = [
    "LLMGateway",
    "LLMGatewayConfigurationError",
    "LLMGatewayError",
    "LLMOutputJSONError",
    "LLMOutputMissingError",
    "LLMOutputValidationError",
    "OpenAILLMGateway",
]
