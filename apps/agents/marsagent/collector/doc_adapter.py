"""官方文档适配器（Wikipedia）。"""
from __future__ import annotations

import asyncio
from datetime import datetime, timezone
from typing import AsyncIterator

import httpx

from .base import RawDoc, SourceAdapter


class WikipediaAdapter(SourceAdapter):
    name = "wikipedia"
    priority = 40
    BASE_URL = "https://en.wikipedia.org/api/rest_v1/page/summary"

    async def search(self, query: str, max_results: int = 5) -> AsyncIterator[RawDoc]:
        slug = query.replace(" ", "_")
        async with httpx.AsyncClient(timeout=10.0) as client:
            try:
                resp = await client.get(f"{self.BASE_URL}/{slug}")
                if resp.status_code != 200:
                    return
                data = resp.json()
                yield RawDoc(
                    url=data.get("content_urls", {}).get("desktop", {}).get("page", ""),
                    title=data.get("title", ""),
                    content=data.get("extract", ""),
                    source=self.name,
                    fetched_at=datetime.now(timezone.utc).isoformat(),
                )
            except Exception:
                pass
