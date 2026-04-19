"""Tests for the Plaid gateway wrapper."""

from __future__ import annotations

import json
from types import SimpleNamespace
from typing import Any

import pytest

import src.backend.gateway.plaid.client as plaid_client_module
from src.backend.gateway.plaid import (
    DEFAULT_PLAID_API_VERSION,
    DEFAULT_PLAID_ENV,
    PlaidGateway,
    PlaidGatewayConfigurationError,
    PlaidGatewayRequestError,
    PlaidSettings,
)


class FakeRequest:
    def __init__(self, **kwargs: Any) -> None:
        self.kwargs = kwargs

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, FakeRequest):
            return NotImplemented
        return self.kwargs == other.kwargs


class FakeResponse:
    def __init__(self, payload: dict[str, Any]) -> None:
        self._payload = payload

    def to_dict(self) -> dict[str, Any]:
        return dict(self._payload)


class RecordingPlaidClient:
    def __init__(self, response_payload: dict[str, Any]) -> None:
        self._response_payload = response_payload
        self.calls: list[tuple[str, Any]] = []

    def __getattr__(self, name: str) -> Any:
        def _call(request: Any) -> FakeResponse:
            self.calls.append((name, request))
            return FakeResponse(self._response_payload)

        return _call


def test_plaid_settings_from_env_reads_expected_values() -> None:
    settings = PlaidSettings.from_env(
        {
            "PLAID_CLIENT_ID": "client-id",
            "PLAID_SECRET": "secret",
            "PLAID_ENV": "production",
            "PLAID_API_VERSION": "2020-09-14",
        }
    )

    assert settings == PlaidSettings(
        client_id="client-id",
        secret="secret",
        environment="production",
        api_version="2020-09-14",
    )


def test_plaid_settings_from_env_uses_defaults_when_optional_values_are_blank() -> None:
    settings = PlaidSettings.from_env(
        {
            "PLAID_CLIENT_ID": "client-id",
            "PLAID_SECRET": "secret",
            "PLAID_ENV": "   ",
            "PLAID_API_VERSION": "   ",
        }
    )

    assert settings.environment == DEFAULT_PLAID_ENV
    assert settings.api_version == DEFAULT_PLAID_API_VERSION


def test_plaid_gateway_initializes_sdk_client_from_settings(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, Any] = {}

    class FakeConfiguration:
        def __init__(self, **kwargs: Any) -> None:
            captured["configuration_kwargs"] = kwargs

    class FakeApiClient:
        def __init__(self, configuration: FakeConfiguration) -> None:
            captured["configuration"] = configuration

    class FakePlaidApi:
        def __init__(self, api_client: FakeApiClient) -> None:
            captured["api_client"] = api_client

    monkeypatch.setattr(
        plaid_client_module,
        "PlaidSDK",
        SimpleNamespace(
            Configuration=FakeConfiguration,
            ApiClient=FakeApiClient,
            Environment=SimpleNamespace(
                Sandbox="https://sandbox.example",
                Production="https://production.example",
            ),
        ),
    )
    monkeypatch.setattr(plaid_client_module, "PlaidApiClass", FakePlaidApi)

    gateway = PlaidGateway(
        settings=PlaidSettings(
            client_id="client-id",
            secret="secret",
            environment="sandbox",
            api_version="2020-09-14",
        )
    )

    assert isinstance(gateway._client, FakePlaidApi)
    assert captured["configuration_kwargs"] == {
        "host": "https://sandbox.example",
        "api_key": {
            "clientId": "client-id",
            "secret": "secret",
            "plaidVersion": "2020-09-14",
        },
    }


def test_plaid_gateway_requires_client_id_without_injected_client() -> None:
    with pytest.raises(PlaidGatewayConfigurationError, match="PLAID_CLIENT_ID"):
        PlaidGateway(
            settings=PlaidSettings(client_id=None, secret="secret"),
        )


def test_plaid_gateway_requires_secret_without_injected_client() -> None:
    with pytest.raises(PlaidGatewayConfigurationError, match="PLAID_SECRET"):
        PlaidGateway(
            settings=PlaidSettings(client_id="client-id", secret=None),
        )


def test_plaid_gateway_rejects_invalid_environment(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        plaid_client_module,
        "PlaidSDK",
        SimpleNamespace(Configuration=object, ApiClient=object),
    )
    monkeypatch.setattr(plaid_client_module, "PlaidApiClass", object)

    with pytest.raises(PlaidGatewayConfigurationError, match="PLAID_ENV"):
        PlaidGateway(
            settings=PlaidSettings(
                client_id="client-id",
                secret="secret",
                environment="development",
            )
        )


def test_create_link_token_builds_request_and_returns_dict(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(plaid_client_module, "LinkTokenCreateRequest", FakeRequest)
    monkeypatch.setattr(plaid_client_module, "LinkTokenCreateRequestUser", FakeRequest)
    monkeypatch.setattr(
        plaid_client_module,
        "Products",
        lambda value: f"product:{value}",
    )
    monkeypatch.setattr(
        plaid_client_module,
        "CountryCode",
        lambda value: f"country:{value}",
    )

    client = RecordingPlaidClient({"link_token": "link-sandbox-token"})
    gateway = PlaidGateway(
        settings=PlaidSettings(client_id=None, secret=None),
        client=client,
    )

    response = gateway.create_link_token(
        client_user_id="user-123",
        client_name="Pypy",
        products=["auth", "transactions"],
        country_codes=["US", "CA"],
        language="en",
        redirect_uri="https://example.com/oauth",
        webhook="https://example.com/webhook",
    )

    assert response == {"link_token": "link-sandbox-token"}
    operation, request = client.calls[0]
    assert operation == "link_token_create"
    assert request.kwargs == {
        "client_name": "Pypy",
        "country_codes": ["country:US", "country:CA"],
        "language": "en",
        "products": ["product:auth", "product:transactions"],
        "redirect_uri": "https://example.com/oauth",
        "user": FakeRequest(client_user_id="user-123"),
        "webhook": "https://example.com/webhook",
    }
    assert request.kwargs["user"].kwargs == {"client_user_id": "user-123"}


@pytest.mark.parametrize(
    (
        "request_model_name",
        "gateway_method",
        "client_operation",
        "call_args",
        "call_kwargs",
        "expected_kwargs",
    ),
    [
        (
            "ItemPublicTokenExchangeRequest",
            "exchange_public_token",
            "item_public_token_exchange",
            ("public-token",),
            {},
            {"public_token": "public-token"},
        ),
        (
            "AuthGetRequest",
            "get_auth",
            "auth_get",
            ("access-token",),
            {},
            {"access_token": "access-token"},
        ),
        (
            "AccountsGetRequest",
            "get_accounts",
            "accounts_get",
            ("access-token",),
            {},
            {"access_token": "access-token"},
        ),
        (
            "TransactionsSyncRequest",
            "sync_transactions",
            "transactions_sync",
            ("access-token",),
            {"cursor": "cursor-123"},
            {"access_token": "access-token", "cursor": "cursor-123"},
        ),
        (
            "ItemGetRequest",
            "get_item",
            "item_get",
            ("access-token",),
            {},
            {"access_token": "access-token"},
        ),
    ],
)
def test_plaid_gateway_methods_build_requests(
    monkeypatch: pytest.MonkeyPatch,
    request_model_name: str,
    gateway_method: str,
    client_operation: str,
    call_args: tuple[Any, ...],
    call_kwargs: dict[str, Any],
    expected_kwargs: dict[str, Any],
) -> None:
    monkeypatch.setattr(plaid_client_module, request_model_name, FakeRequest)

    client = RecordingPlaidClient({"ok": True})
    gateway = PlaidGateway(
        settings=PlaidSettings(client_id=None, secret=None),
        client=client,
    )

    response = getattr(gateway, gateway_method)(*call_args, **call_kwargs)

    assert response == {"ok": True}
    operation, request = client.calls[0]
    assert operation == client_operation
    assert request.kwargs == expected_kwargs


def test_plaid_gateway_wraps_plaid_api_errors(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class FakeApiException(Exception):
        def __init__(self) -> None:
            super().__init__("request failed")
            self.body = json.dumps(
                {
                    "error_code": "INVALID_PUBLIC_TOKEN",
                    "error_type": "INVALID_INPUT",
                    "error_message": "public token is invalid",
                }
            )
            self.status = 400

    class ErroringPlaidClient:
        def item_public_token_exchange(self, request: Any) -> dict[str, Any]:
            raise FakeApiException()

    monkeypatch.setattr(
        plaid_client_module,
        "ItemPublicTokenExchangeRequest",
        FakeRequest,
    )
    monkeypatch.setattr(plaid_client_module, "PlaidApiException", FakeApiException)

    gateway = PlaidGateway(
        settings=PlaidSettings(client_id=None, secret=None),
        client=ErroringPlaidClient(),
    )

    with pytest.raises(PlaidGatewayRequestError) as exc_info:
        gateway.exchange_public_token("public-token")

    assert exc_info.value.operation == "item_public_token_exchange"
    assert exc_info.value.error_code == "INVALID_PUBLIC_TOKEN"
    assert exc_info.value.error_type == "INVALID_INPUT"
    assert exc_info.value.status_code == 400
    assert "public token is invalid" in str(exc_info.value)
