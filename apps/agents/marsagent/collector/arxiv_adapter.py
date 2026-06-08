"""arXiv 论文适配器。"""
from __future__ import annotations

import asyncio
from datetime import datetime, timezone
from typing import AsyncIterator

import arxiv

from .base import RawDoc, SourceAdapter


class ArxivAdapter(SourceAdapter):
    name = "arxiv"
    priority = 20

    def __init__(self) -> None:
        self.client = arxiv.Client()

    async def search(self, query: str, max_results: int = 10) -> AsyncIterator[RawDoc]:
        loop = asyncio.get_event_loop()
        search = arxiv.Search(
            query=query, max_results=max_results,
            sort_by=arxiv.SortStrategy.Relevance,
        )
        results = await loop.run_in_executor(
            None,
            lambda: list(self.client.results(search)),
        )
        for result in results:
            yield RawDoc(
                url=result.entry_id or "",
                title=result.title or "",
                content=f"{result.summary}\n\nComments: {result.comment or ''}",
                source=self.name,
                fetched_at=datetime.now(timezone.utc).isoformat(),
            )
