"""Tavily 搜索适配器。"""
from __future__ import annotations

import asyncio
import os
from datetime import datetime, timezone
from typing import AsyncIterator

from tavily import TavilyClient

from .base import RawDoc, SourceAdapter


class TavilyAdapter(SourceAdapter):
    name = "tavily"
    priority = 10

    def __init__(self) -> None:
        api_key = os.getenv("TAVILY_API_KEY", "")
        if not api_key:
            raise RuntimeError("TAVILY_API_KEY not set")
        self.client = TavilyClient(api_key=api_key)

    async def search(self, query: str, max_results: int = 10) -> AsyncIterator[RawDoc]:
        loop = asyncio.get_event_loop()
        result = await loop.run_in_executor(
            None,
            lambda: self.client.search(
                query=query, max_results=max_results, include_answer=True
            ),
        )
        for item in (result.get("results") or []):
            yield RawDoc(
                url=item.get("url", ""),
                title=item.get("title", ""),
                content=item.get("content", ""),
                source=self.name,
                fetched_at=datetime.now(timezone.utc).isoformat(),
            )
