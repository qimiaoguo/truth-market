"""Tests for the Truth Market WebSocket client."""

import asyncio
import json
import pytest

from truthmarket.ws import TruthMarketWS


async def _run_ws_server(handler, host="127.0.0.1", port=0):
    """Start a WebSocket server and return (server, port)."""
    import websockets

    server = await websockets.serve(handler, host, port)
    port = server.sockets[0].getsockname()[1]
    return server, port


async def test_subscribes_to_market():
    """WS client sends a subscribe message for a market channel."""
    received_messages: list[dict] = []

    async def handler(websocket):
        async for raw in websocket:
            received_messages.append(json.loads(raw))

    server, port = await _run_ws_server(handler)

    try:
        ws = TruthMarketWS(url=f"ws://127.0.0.1:{port}")
        await ws.connect()
        await ws.subscribe_market("market-1")

        # Give the server a moment to process the message.
        await asyncio.sleep(0.1)

        assert len(received_messages) == 1
        assert received_messages[0] == {
            "type": "subscribe",
            "channel": "market:market-1",
        }
    finally:
        await ws.close()
        server.close()
        await server.wait_closed()


async def test_receives_trade_events():
    """WS client fires on_event callback when a trade event arrives."""
    received_events: list[dict] = []

    async def handler(websocket):
        # Wait for the subscribe message from the client.
        raw = await websocket.recv()
        msg = json.loads(raw)
        assert msg["type"] == "subscribe"

        # Send a trade event back to the client.
        await websocket.send(
            json.dumps(
                {
                    "type": "event",
                    "channel": "market:market-1",
                    "payload": {"trade_id": "t-1", "price": "0.55"},
                }
            )
        )

    server, port = await _run_ws_server(handler)

    try:
        ws = TruthMarketWS(url=f"ws://127.0.0.1:{port}")

        def on_event(event: dict) -> None:
            received_events.append(event)

        ws.on_event = on_event

        await ws.connect()
        await ws.subscribe_market("market-1")

        # Give time for the server to send the event and the client to process it.
        await asyncio.sleep(0.3)

        assert len(received_events) == 1
        assert received_events[0]["channel"] == "market:market-1"
        assert received_events[0]["payload"]["trade_id"] == "t-1"
        assert received_events[0]["payload"]["price"] == "0.55"
    finally:
        await ws.close()
        server.close()
        await server.wait_closed()
