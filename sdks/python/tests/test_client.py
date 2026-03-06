"""Tests for the Truth Market HTTP client."""

import respx
import httpx
import pytest

from truthmarket.client import TruthMarketClient, TruthMarketError


@respx.mock
def test_authenticates_with_api_key():
    """Client sends X-API-Key header and returns user from /auth/me."""
    route = respx.get("http://localhost:8080/api/v1/auth/me").mock(
        return_value=httpx.Response(
            200,
            json={
                "ok": True,
                "data": {
                    "user": {
                        "id": "user-1",
                        "wallet_address": "0xabc",
                        "user_type": "agent",
                    }
                },
            },
        )
    )

    client = TruthMarketClient(
        base_url="http://localhost:8080/api/v1",
        api_key="tm_test_key_12345",
    )
    result = client.get_me()

    assert result["user"]["id"] == "user-1"
    assert result["user"]["wallet_address"] == "0xabc"
    assert result["user"]["user_type"] == "agent"
    assert route.calls[0].request.headers["X-API-Key"] == "tm_test_key_12345"


@respx.mock
def test_lists_markets():
    """Client returns markets list with pagination meta from GET /markets."""
    respx.get("http://localhost:8080/api/v1/markets").mock(
        return_value=httpx.Response(
            200,
            json={
                "ok": True,
                "data": {
                    "markets": [
                        {"id": "m-1", "title": "Test?", "status": "open"}
                    ]
                },
                "meta": {"page": 1, "per_page": 20, "total": 1},
            },
        )
    )

    client = TruthMarketClient(
        base_url="http://localhost:8080/api/v1",
        api_key="tm_test_key_12345",
    )
    result = client.list_markets()

    assert len(result["markets"]) == 1
    assert result["markets"][0]["id"] == "m-1"
    assert result["markets"][0]["title"] == "Test?"
    assert result["markets"][0]["status"] == "open"
    assert result["meta"]["page"] == 1
    assert result["meta"]["per_page"] == 20
    assert result["meta"]["total"] == 1


@respx.mock
def test_places_order():
    """Client sends order with snake_case fields and returns created order."""
    route = respx.post("http://localhost:8080/api/v1/trading/orders").mock(
        return_value=httpx.Response(
            200,
            json={
                "ok": True,
                "data": {
                    "order": {
                        "id": "order-1",
                        "side": "buy",
                        "price": "0.65",
                        "quantity": "50",
                        "status": "open",
                    }
                },
            },
        )
    )

    client = TruthMarketClient(
        base_url="http://localhost:8080/api/v1",
        api_key="tm_test_key_12345",
    )
    result = client.place_order(
        market_id="m-1",
        outcome_id="o-1",
        side="buy",
        price="0.65",
        quantity="50",
    )

    assert result["order"]["id"] == "order-1"
    assert result["order"]["side"] == "buy"
    assert result["order"]["price"] == "0.65"
    assert result["order"]["quantity"] == "50"
    assert result["order"]["status"] == "open"

    request_body = route.calls[0].request.content
    import json

    body = json.loads(request_body)
    assert body["market_id"] == "m-1"
    assert body["outcome_id"] == "o-1"
    assert body["side"] == "buy"
    assert body["price"] == "0.65"
    assert body["quantity"] == "50"


@respx.mock
def test_cancels_order():
    """Client sends DELETE to cancel an order and returns cancelled order."""
    respx.delete("http://localhost:8080/api/v1/trading/orders/order-1").mock(
        return_value=httpx.Response(
            200,
            json={
                "ok": True,
                "data": {
                    "order": {
                        "id": "order-1",
                        "status": "cancelled",
                    }
                },
            },
        )
    )

    client = TruthMarketClient(
        base_url="http://localhost:8080/api/v1",
        api_key="tm_test_key_12345",
    )
    result = client.cancel_order("order-1")

    assert result["order"]["id"] == "order-1"
    assert result["order"]["status"] == "cancelled"


@respx.mock
def test_gets_positions():
    """Client returns trading positions from GET /trading/positions."""
    respx.get("http://localhost:8080/api/v1/trading/positions").mock(
        return_value=httpx.Response(
            200,
            json={
                "ok": True,
                "data": {
                    "positions": [
                        {
                            "id": "pos-1",
                            "market_id": "m-1",
                            "outcome_id": "o-1",
                            "quantity": "100",
                            "avg_price": "0.50",
                        }
                    ]
                },
            },
        )
    )

    client = TruthMarketClient(
        base_url="http://localhost:8080/api/v1",
        api_key="tm_test_key_12345",
    )
    result = client.get_positions()

    assert len(result["positions"]) == 1
    assert result["positions"][0]["id"] == "pos-1"
    assert result["positions"][0]["market_id"] == "m-1"
    assert result["positions"][0]["outcome_id"] == "o-1"
    assert result["positions"][0]["quantity"] == "100"
    assert result["positions"][0]["avg_price"] == "0.50"


@respx.mock
def test_handles_rate_limit():
    """Client retries on 429 and succeeds on the subsequent attempt."""
    route = respx.get("http://localhost:8080/api/v1/markets")
    route.side_effect = [
        httpx.Response(
            429,
            json={
                "ok": False,
                "error": {
                    "code": "RATE_LIMITED",
                    "message": "Too many requests",
                },
            },
        ),
        httpx.Response(
            200,
            json={
                "ok": True,
                "data": {
                    "markets": [
                        {"id": "m-1", "title": "Test?", "status": "open"}
                    ]
                },
                "meta": {"page": 1, "per_page": 20, "total": 1},
            },
        ),
    ]

    client = TruthMarketClient(
        base_url="http://localhost:8080/api/v1",
        api_key="tm_test_key_12345",
        max_retries=2,
    )
    result = client.list_markets()

    assert len(result["markets"]) == 1
    assert result["markets"][0]["id"] == "m-1"
    assert route.call_count == 2
