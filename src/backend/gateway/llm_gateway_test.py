"""Tests for the OpenAI-backed structured LLM gateway."""

import asyncio
from types import SimpleNamespace
from typing import Any

import pytest
from pydantic import BaseModel

import src.backend.gateway.llm_gateway as llm_gateway_module
from src.backend.gateway.llm_gateway import (
    LLMGatewayConfigurationError,
    LLMOutputJSONError,
    LLMOutputMissingError,
    LLMOutputValidationError,
    OpenAILLMGateway,
)
from src.backend.settings import OpenAISettings


class ExampleStructuredResponse(BaseModel):
    answer: str
    confidence: float


class StubResponsesClient:
    def __init__(self, response: object) -> None:
        self._response = response
        self.calls: list[dict[str, Any]] = []

    async def create(self, **kwargs: Any) -> object:
        self.calls.append(kwargs)
        return self._response


class StubOpenAIClient:
    def __init__(self, response: object) -> None:
        self.responses = StubResponsesClient(response)


def test_openai_gateway_initializes_async_client_from_settings(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured_kwargs: dict[str, object] = {}

    class FakeAsyncOpenAI:
        def __init__(self, **kwargs: object) -> None:
            captured_kwargs.update(kwargs)
            self.responses = StubResponsesClient(SimpleNamespace())

    monkeypatch.setattr(llm_gateway_module, "AsyncOpenAI", FakeAsyncOpenAI)

    gateway = OpenAILLMGateway(
        settings=OpenAISettings(
            api_key="sk-test",
            model="gpt-test",
            base_url="https://example.com/v1",
            timeout_s=12,
            max_retries=4,
        )
    )

    assert gateway.last_response_id is None
    assert captured_kwargs == {
        "api_key": "sk-test",
        "base_url": "https://example.com/v1",
        "timeout": 12.0,
        "max_retries": 4,
    }


def test_openai_gateway_requires_model_configuration() -> None:
    with pytest.raises(LLMGatewayConfigurationError, match="OPENAI_MODEL"):
        OpenAILLMGateway(
            settings=OpenAISettings(api_key="sk-test", model=None),
            client=object(),
        )


def test_openai_gateway_requires_api_key_without_injected_client() -> None:
    with pytest.raises(LLMGatewayConfigurationError, match="OPENAI_API_KEY"):
        OpenAILLMGateway(settings=OpenAISettings(api_key=None, model="gpt-test"))


def test_structured_completion_uses_schema_and_returns_validated_model() -> None:
    response = SimpleNamespace(
        id="resp_123",
        output=[
            SimpleNamespace(
                content=[
                    SimpleNamespace(
                        type="output_text",
                        text='{"answer": "42", "confidence": 0.9}',
                    )
                ]
            )
        ],
    )
    client = StubOpenAIClient(response)
    gateway = OpenAILLMGateway(
        settings=OpenAISettings(api_key=None, model="gpt-test"),
        client=client,
    )

    result = asyncio.run(
        gateway.structured_completion(
            system_prompt="Return structured data only.",
            user_prompt="What is the answer?",
            response_model=ExampleStructuredResponse,
        )
    )

    assert result == ExampleStructuredResponse(answer="42", confidence=0.9)
    assert gateway.last_response_id == "resp_123"

    request = client.responses.calls[0]
    assert request["model"] == "gpt-test"
    assert request["input"] == [
        {"role": "system", "content": "Return structured data only."},
        {"role": "user", "content": "What is the answer?"},
    ]
    assert request["text"]["format"]["type"] == "json_schema"
    assert request["text"]["format"]["name"] == "ExampleStructuredResponse"
    assert request["text"]["format"]["strict"] is True
    assert (
        request["text"]["format"]["schema"]["properties"]
        == ExampleStructuredResponse.model_json_schema()["properties"]
    )
    assert (
        request["text"]["format"]["schema"]["required"]
        == ExampleStructuredResponse.model_json_schema()["required"]
    )


def test_structured_completion_rejects_invalid_json() -> None:
    client = StubOpenAIClient(
        SimpleNamespace(
            id="resp_bad_json",
            output_text='{"answer": "42", "confidence": 0.9',
        )
    )
    gateway = OpenAILLMGateway(
        settings=OpenAISettings(api_key=None, model="gpt-test"),
        client=client,
    )

    with pytest.raises(LLMOutputJSONError, match="not valid JSON"):
        asyncio.run(
            gateway.structured_completion(
                system_prompt="Return structured data only.",
                user_prompt="What is the answer?",
                response_model=ExampleStructuredResponse,
            )
        )


def test_structured_completion_rejects_schema_invalid_json() -> None:
    client = StubOpenAIClient(
        SimpleNamespace(
            id="resp_bad_schema",
            output_text='{"answer": "42"}',
        )
    )
    gateway = OpenAILLMGateway(
        settings=OpenAISettings(api_key=None, model="gpt-test"),
        client=client,
    )

    with pytest.raises(LLMOutputValidationError, match="expected schema"):
        asyncio.run(
            gateway.structured_completion(
                system_prompt="Return structured data only.",
                user_prompt="What is the answer?",
                response_model=ExampleStructuredResponse,
            )
        )


def test_structured_completion_rejects_missing_output_text() -> None:
    client = StubOpenAIClient(
        SimpleNamespace(
            id="resp_no_text",
            output=[SimpleNamespace(content=[SimpleNamespace(type="refusal")])],
        )
    )
    gateway = OpenAILLMGateway(
        settings=OpenAISettings(api_key=None, model="gpt-test"),
        client=client,
    )

    with pytest.raises(LLMOutputMissingError, match="did not include any output text"):
        asyncio.run(
            gateway.structured_completion(
                system_prompt="Return structured data only.",
                user_prompt="What is the answer?",
                response_model=ExampleStructuredResponse,
            )
        )
