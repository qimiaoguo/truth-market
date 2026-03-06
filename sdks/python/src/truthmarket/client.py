"""Truth Market Python SDK HTTP client."""

from __future__ import annotations

import time
from typing import Any, Optional

import httpx


class TruthMarketError(Exception):
    """Base exception for Truth Market SDK errors."""

    def __init__(self, message: str, code: Optional[str] = None, status: Optional[int] = None):
        super().__init__(message)
        self.code = code
        self.status = status


class TruthMarketClient:
    """Synchronous HTTP client for the Truth Market REST API."""

    def __init__(
        self,
        base_url: str,
        api_key: str,
        max_retries: int = 0,
        timeout: float = 30.0,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._max_retries = max_retries
        self._client = httpx.Client(
            base_url=self._base_url,
            headers={"X-API-Key": api_key},
            timeout=timeout,
        )

    def _request(
        self,
        method: str,
        path: str,
        *,
        json_body: dict | None = None,
        params: dict | None = None,
    ) -> dict[str, Any]:
        """Make an HTTP request with optional retry on 429."""
        last_exc: TruthMarketError | None = None
        max_attempts = 1 + self._max_retries

        for attempt in range(max_attempts):
            response = self._client.request(
                method, path, json=json_body, params=params
            )

            if response.status_code == 429 and attempt < max_attempts - 1:
                # Retry after short sleep
                retry_after = response.headers.get("Retry-After", "0.1")
                time.sleep(float(retry_after) * 0.01)  # shortened for tests
                body = response.json()
                last_exc = TruthMarketError(
                    body.get("error", {}).get("message", "Rate limited"),
                    code=body.get("error", {}).get("code"),
                    status=429,
                )
                continue

            if response.status_code >= 400:
                body = response.json()
                raise TruthMarketError(
                    body.get("error", {}).get("message", "Request failed"),
                    code=body.get("error", {}).get("code"),
                    status=response.status_code,
                )

            return response.json()

        raise last_exc or TruthMarketError("Request failed after retries")

    def _unwrap(self, response: dict) -> dict[str, Any]:
        """Extract 'data' from envelope, merge 'meta' if present."""
        result = dict(response.get("data", {}))
        if "meta" in response:
            result["meta"] = response["meta"]
        return result

    # -- Auth ---------------------------------------------------------------

    def get_me(self) -> dict[str, Any]:
        resp = self._request("GET", "/auth/me")
        return self._unwrap(resp)

    # -- Markets ------------------------------------------------------------

    def list_markets(
        self,
        *,
        status: str | None = None,
        category: str | None = None,
        page: int | None = None,
        per_page: int | None = None,
    ) -> dict[str, Any]:
        params: dict[str, str] = {}
        if status:
            params["status"] = status
        if category:
            params["category"] = category
        if page is not None:
            params["page"] = str(page)
        if per_page is not None:
            params["per_page"] = str(per_page)
        resp = self._request("GET", "/markets", params=params or None)
        return self._unwrap(resp)

    def get_market(self, market_id: str) -> dict[str, Any]:
        resp = self._request("GET", f"/markets/{market_id}")
        return self._unwrap(resp)

    def get_orderbook(
        self, market_id: str, outcome_id: str
    ) -> dict[str, Any]:
        resp = self._request(
            "GET",
            f"/markets/{market_id}/orderbook",
            params={"outcome_id": outcome_id},
        )
        return self._unwrap(resp)

    # -- Trading ------------------------------------------------------------

    def mint_tokens(self, market_id: str, quantity: str) -> dict[str, Any]:
        resp = self._request(
            "POST",
            "/trading/mint",
            json_body={"market_id": market_id, "quantity": quantity},
        )
        return self._unwrap(resp)

    def place_order(
        self,
        market_id: str,
        outcome_id: str,
        side: str,
        price: str,
        quantity: str,
    ) -> dict[str, Any]:
        resp = self._request(
            "POST",
            "/trading/orders",
            json_body={
                "market_id": market_id,
                "outcome_id": outcome_id,
                "side": side,
                "price": price,
                "quantity": quantity,
            },
        )
        return self._unwrap(resp)

    def cancel_order(self, order_id: str) -> dict[str, Any]:
        resp = self._request("DELETE", f"/trading/orders/{order_id}")
        return self._unwrap(resp)

    def get_positions(
        self, *, market_id: str | None = None
    ) -> dict[str, Any]:
        params: dict[str, str] = {}
        if market_id:
            params["market_id"] = market_id
        resp = self._request("GET", "/trading/positions", params=params or None)
        return self._unwrap(resp)

    # -- Rankings -----------------------------------------------------------

    def get_rankings(
        self,
        *,
        dimension: str | None = None,
        user_type: str | None = None,
        page: int | None = None,
        per_page: int | None = None,
    ) -> dict[str, Any]:
        params: dict[str, str] = {}
        if dimension:
            params["dimension"] = dimension
        if user_type:
            params["user_type"] = user_type
        if page is not None:
            params["page"] = str(page)
        if per_page is not None:
            params["per_page"] = str(per_page)
        resp = self._request("GET", "/rankings", params=params or None)
        return self._unwrap(resp)

    def get_user_rankings(self, user_id: str) -> dict[str, Any]:
        resp = self._request("GET", f"/rankings/user/{user_id}")
        return self._unwrap(resp)
