"""Truth Market Python SDK WebSocket client."""

from __future__ import annotations

import asyncio
import json
from typing import Any, Callable, Optional

import websockets


class TruthMarketWS:
    """Async WebSocket client for real-time Truth Market events."""

    def __init__(self, url: str) -> None:
        self._url = url
        self._ws: Any = None  # websockets connection
        self._listen_task: asyncio.Task | None = None
        self.on_event: Optional[Callable[[dict[str, Any]], None]] = None

    async def connect(self) -> None:
        self._ws = await websockets.connect(self._url)
        self._listen_task = asyncio.create_task(self._listen())

    async def _listen(self) -> None:
        try:
            async for raw in self._ws:
                msg = json.loads(raw)
                if self.on_event and msg.get("type") == "event":
                    self.on_event(msg)
        except websockets.ConnectionClosed:
            pass

    async def subscribe_market(self, market_id: str) -> None:
        await self._ws.send(
            json.dumps({"type": "subscribe", "channel": f"market:{market_id}"})
        )

    async def close(self) -> None:
        if self._listen_task:
            self._listen_task.cancel()
            try:
                await self._listen_task
            except asyncio.CancelledError:
                pass
        if self._ws:
            await self._ws.close()
