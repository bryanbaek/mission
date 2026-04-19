"""Plaid gateway wrapper for baseline auth, accounts, and transactions flows."""

from __future__ import annotations

import json
import os
from collections.abc import Mapping, Sequence
from typing import Any

from pydantic import BaseModel, ConfigDict, Field

DEFAULT_PLAID_ENV = "sandbox"
DEFAULT_PLAID_API_VERSION = "2020-09-14"
PLAID_ENVIRONMENT_HOSTS = {
    "sandbox": "https://sandbox.plaid.com",
    "production": "https://production.plaid.com",
}

PlaidSDK: Any | None
PlaidApiClass: Any | None
PlaidApiException: type[BaseException] | None
AuthGetRequest: Any | None
AccountsGetRequest: Any | None
CountryCode: Any | None
ItemGetRequest: Any | None
ItemPublicTokenExchangeRequest: Any | None
LinkTokenCreateRequest: Any | None
LinkTokenCreateRequestUser: Any | None
Products: Any | None
TransactionsSyncRequest: Any | None

try:
    import plaid as _PlaidSDK
    from plaid.api import plaid_api as _plaid_api
    from plaid.model.accounts_get_request import (
        AccountsGetRequest as _AccountsGetRequest,
    )
    from plaid.model.auth_get_request import AuthGetRequest as _AuthGetRequest
    from plaid.model.country_code import CountryCode as _CountryCode
    from plaid.model.item_get_request import ItemGetRequest as _ItemGetRequest
    from plaid.model.item_public_token_exchange_request import (
        ItemPublicTokenExchangeRequest as _ItemPublicTokenExchangeRequest,
    )
    from plaid.model.link_token_create_request import (
        LinkTokenCreateRequest as _LinkTokenCreateRequest,
    )
    from plaid.model.link_token_create_request_user import (
        LinkTokenCreateRequestUser as _LinkTokenCreateRequestUser,
    )
    from plaid.model.products import Products as _Products
    from plaid.model.transactions_sync_request import (
        TransactionsSyncRequest as _TransactionsSyncRequest,
    )
except ImportError:  # pragma: no cover - depends on local environment setup
    PlaidSDK = None
    PlaidApiClass = None
    PlaidApiException = None
    AuthGetRequest = None
    AccountsGetRequest = None
    CountryCode = None
    ItemGetRequest = None
    ItemPublicTokenExchangeRequest = None
    LinkTokenCreateRequest = None
    LinkTokenCreateRequestUser = None
    Products = None
    TransactionsSyncRequest = None
else:
    PlaidSDK = _PlaidSDK
    PlaidApiClass = _plaid_api.PlaidApi
    PlaidApiException = _PlaidSDK.ApiException
    AuthGetRequest = _AuthGetRequest
    AccountsGetRequest = _AccountsGetRequest
    CountryCode = _CountryCode
    ItemGetRequest = _ItemGetRequest
    ItemPublicTokenExchangeRequest = _ItemPublicTokenExchangeRequest
    LinkTokenCreateRequest = _LinkTokenCreateRequest
    LinkTokenCreateRequestUser = _LinkTokenCreateRequestUser
    Products = _Products
    TransactionsSyncRequest = _TransactionsSyncRequest


def _read_optional_env(
    environ: Mapping[str, str],
    key: str,
    *,
    default: str | None = None,
) -> str | None:
    value = environ.get(key)
    if value is None:
        return default

    normalized = value.strip()
    if not normalized:
        return default
    return normalized


class PlaidGatewayError(RuntimeError):
    """Base error for Plaid gateway failures."""


class PlaidGatewayConfigurationError(PlaidGatewayError):
    """Raised when Plaid gateway configuration is incomplete or invalid."""


class PlaidGatewayRequestError(PlaidGatewayError):
    """Raised when Plaid returns an API error."""

    def __init__(
        self,
        operation: str,
        message: str,
        *,
        error_code: str | None = None,
        error_type: str | None = None,
        status_code: int | None = None,
    ) -> None:
        super().__init__(message)
        self.operation = operation
        self.error_code = error_code
        self.error_type = error_type
        self.status_code = status_code


class PlaidSettings(BaseModel):
    """Configuration required by the Plaid gateway."""

    model_config = ConfigDict(frozen=True)

    client_id: str | None = None
    secret: str | None = None
    environment: str = Field(default=DEFAULT_PLAID_ENV, min_length=1)
    api_version: str = Field(default=DEFAULT_PLAID_API_VERSION, min_length=1)

    @classmethod
    def from_env(cls, environ: Mapping[str, str] | None = None) -> "PlaidSettings":
        """Build Plaid settings from environment variables."""
        source = os.environ if environ is None else environ
        return cls.model_validate(
            {
                "client_id": _read_optional_env(source, "PLAID_CLIENT_ID"),
                "secret": _read_optional_env(source, "PLAID_SECRET"),
                "environment": _read_optional_env(
                    source,
                    "PLAID_ENV",
                    default=DEFAULT_PLAID_ENV,
                ),
                "api_version": _read_optional_env(
                    source,
                    "PLAID_API_VERSION",
                    default=DEFAULT_PLAID_API_VERSION,
                ),
            }
        )


class PlaidGateway:
    """Small Plaid client wrapper for finance-domain gateway usage."""

    def __init__(
        self,
        *,
        settings: PlaidSettings | None = None,
        client: Any | None = None,
    ) -> None:
        resolved_settings = PlaidSettings.from_env() if settings is None else settings
        if client is None and not resolved_settings.client_id:
            raise PlaidGatewayConfigurationError(
                "PLAID_CLIENT_ID is required to initialize PlaidGateway "
                "when no client is injected."
            )
        if client is None and not resolved_settings.secret:
            raise PlaidGatewayConfigurationError(
                "PLAID_SECRET is required to initialize PlaidGateway "
                "when no client is injected."
            )

        self._settings = resolved_settings
        self._client = client if client is not None else self._build_client()

    def create_link_token(
        self,
        *,
        client_user_id: str,
        client_name: str,
        products: Sequence[str],
        country_codes: Sequence[str] = ("US",),
        language: str = "en",
        redirect_uri: str | None = None,
        webhook: str | None = None,
    ) -> dict[str, Any]:
        request_model = self._require_sdk_dependency(
            LinkTokenCreateRequest,
            "LinkTokenCreateRequest",
        )
        request_user_model = self._require_sdk_dependency(
            LinkTokenCreateRequestUser,
            "LinkTokenCreateRequestUser",
        )
        product_model = self._require_sdk_dependency(Products, "Products")
        country_code_model = self._require_sdk_dependency(CountryCode, "CountryCode")

        request_kwargs: dict[str, Any] = {
            "client_name": client_name,
            "country_codes": [
                country_code_model(country_code)
                for country_code in country_codes
            ],
            "language": language,
            "products": [product_model(product) for product in products],
            "user": request_user_model(client_user_id=client_user_id),
        }
        if redirect_uri is not None:
            request_kwargs["redirect_uri"] = redirect_uri
        if webhook is not None:
            request_kwargs["webhook"] = webhook

        request = request_model(**request_kwargs)
        return self._execute("link_token_create", request)

    def exchange_public_token(self, public_token: str) -> dict[str, Any]:
        request_model = self._require_sdk_dependency(
            ItemPublicTokenExchangeRequest,
            "ItemPublicTokenExchangeRequest",
        )
        request = request_model(public_token=public_token)
        return self._execute("item_public_token_exchange", request)

    def get_auth(self, access_token: str) -> dict[str, Any]:
        request_model = self._require_sdk_dependency(AuthGetRequest, "AuthGetRequest")
        request = request_model(access_token=access_token)
        return self._execute("auth_get", request)

    def get_accounts(self, access_token: str) -> dict[str, Any]:
        request_model = self._require_sdk_dependency(
            AccountsGetRequest,
            "AccountsGetRequest",
        )
        request = request_model(access_token=access_token)
        return self._execute("accounts_get", request)

    def sync_transactions(
        self,
        access_token: str,
        *,
        cursor: str | None = None,
    ) -> dict[str, Any]:
        request_model = self._require_sdk_dependency(
            TransactionsSyncRequest,
            "TransactionsSyncRequest",
        )
        request_kwargs: dict[str, Any] = {"access_token": access_token}
        if cursor is not None:
            request_kwargs["cursor"] = cursor
        request = request_model(**request_kwargs)
        return self._execute("transactions_sync", request)

    def get_item(self, access_token: str) -> dict[str, Any]:
        request_model = self._require_sdk_dependency(ItemGetRequest, "ItemGetRequest")
        request = request_model(access_token=access_token)
        return self._execute("item_get", request)

    def _build_client(self) -> Any:
        host = self._resolve_host(self._settings.environment)
        sdk = self._require_sdk_dependency(PlaidSDK, "plaid-python")
        plaid_api_class = self._require_sdk_dependency(PlaidApiClass, "PlaidApi")

        configuration = sdk.Configuration(
            host=host,
            api_key={
                "clientId": self._settings.client_id,
                "secret": self._settings.secret,
                "plaidVersion": self._settings.api_version,
            },
        )
        api_client = sdk.ApiClient(configuration)
        return plaid_api_class(api_client)

    def _resolve_host(self, environment: str) -> str:
        normalized_environment = environment.strip().lower()
        if normalized_environment == "sandbox":
            sdk_environment = getattr(
                getattr(PlaidSDK, "Environment", None),
                "Sandbox",
                None,
            )
            return (
                sdk_environment
                if isinstance(sdk_environment, str)
                else PLAID_ENVIRONMENT_HOSTS["sandbox"]
            )
        if normalized_environment == "production":
            sdk_environment = getattr(
                getattr(PlaidSDK, "Environment", None),
                "Production",
                None,
            )
            return (
                sdk_environment
                if isinstance(sdk_environment, str)
                else PLAID_ENVIRONMENT_HOSTS["production"]
            )
        supported = ", ".join(sorted(PLAID_ENVIRONMENT_HOSTS))
        raise PlaidGatewayConfigurationError(
            f"PLAID_ENV must be one of: {supported}."
        )

    def _require_sdk_dependency(self, dependency: Any, name: str) -> Any:
        if dependency is None:
            raise PlaidGatewayConfigurationError(
                "The plaid-python package must be installed to use "
                f"PlaidGateway ({name} is unavailable)."
            )
        return dependency

    def _execute(self, operation: str, request: Any) -> dict[str, Any]:
        method = getattr(self._client, operation)
        try:
            response = method(request)
        except Exception as exc:
            if PlaidApiException is not None and isinstance(exc, PlaidApiException):
                raise self._build_request_error(operation, exc) from exc
            raise
        return self._normalize_response(response)

    def _build_request_error(
        self,
        operation: str,
        exc: BaseException,
    ) -> PlaidGatewayRequestError:
        status_code = getattr(exc, "status", None)
        error_code: str | None = None
        error_type: str | None = None
        provider_message: str | None = None

        body = getattr(exc, "body", None)
        if isinstance(body, str) and body.strip():
            try:
                payload = json.loads(body)
            except json.JSONDecodeError:
                payload = None
            if isinstance(payload, Mapping):
                error_code = self._read_string(payload, "error_code")
                error_type = self._read_string(payload, "error_type")
                provider_message = self._read_string(payload, "error_message")

        message = provider_message or str(exc) or "Plaid request failed."
        details: list[str] = []
        if error_code:
            details.append(f"error_code={error_code}")
        if error_type:
            details.append(f"error_type={error_type}")
        if isinstance(status_code, int):
            details.append(f"status={status_code}")
        detail_suffix = f" ({', '.join(details)})" if details else ""

        return PlaidGatewayRequestError(
            operation,
            f"Plaid request {operation} failed: {message}{detail_suffix}",
            error_code=error_code,
            error_type=error_type,
            status_code=status_code if isinstance(status_code, int) else None,
        )

    def _normalize_response(self, response: Any) -> dict[str, Any]:
        if isinstance(response, Mapping):
            return dict(response)

        to_dict = getattr(response, "to_dict", None)
        if callable(to_dict):
            normalized = to_dict()
            if isinstance(normalized, Mapping):
                return dict(normalized)

        raise PlaidGatewayError(
            "PlaidGateway received a response that could not be normalized to a dict."
        )

    def _read_string(
        self,
        payload: Mapping[str, Any],
        key: str,
    ) -> str | None:
        value = payload.get(key)
        if isinstance(value, str) and value:
            return value
        return None


def get_plaid_settings(environ: Mapping[str, str] | None = None) -> PlaidSettings:
    """Return the repository's Plaid settings object."""
    return PlaidSettings.from_env(environ)


__all__ = [
    "DEFAULT_PLAID_API_VERSION",
    "DEFAULT_PLAID_ENV",
    "PLAID_ENVIRONMENT_HOSTS",
    "PlaidGateway",
    "PlaidGatewayConfigurationError",
    "PlaidGatewayError",
    "PlaidGatewayRequestError",
    "PlaidSettings",
    "get_plaid_settings",
]
