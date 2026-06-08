"""官方文档/百科适配器（Wikipedia search + summary）。"""
from __future__ import annotations

from datetime import datetime, timezone
from typing import AsyncIterator
from urllib.parse import quote

import httpx

from .base import RawDoc, SourceAdapter


class WikipediaAdapter(SourceAdapter):
    name = "wikipedia"
    priority = 40
    SEARCH_URL = "https://en.wikipedia.org/w/api.php"
    SUMMARY_URL = "https://en.wikipedia.org/api/rest_v1/page/summary"

    async def search(self, query: str, max_results: int = 5) -> AsyncIterator[RawDoc]:
        async with httpx.AsyncClient(timeout=15.0, follow_redirects=True) as client:
            titles = await self._search_titles(client, query, max_results)
            if not titles:
                async for doc in self._github_fallback(client, query, max_results):
                    yield doc
                return
            yielded = 0
            for title in titles:
                if yielded >= max_results:
                    break
                doc = await self._summary_doc(client, title)
                if doc is None:
                    continue
                yielded += 1
                yield doc
            if yielded == 0:
                async for doc in self._github_fallback(client, query, max_results):
                    yield doc

    async def _search_titles(self, client: httpx.AsyncClient, query: str, max_results: int) -> list[str]:
        try:
            resp = await client.get(
                self.SEARCH_URL,
                params={
                    "action": "query",
                    "list": "search",
                    "srsearch": query,
                    "srlimit": max_results,
                    "format": "json",
                },
            )
            resp.raise_for_status()
            data = resp.json()
            return [item.get("title", "") for item in data.get("query", {}).get("search", []) if item.get("title")]
        except Exception:
            return []

    async def _github_fallback(
        self, client: httpx.AsyncClient, query: str, max_results: int,
    ) -> AsyncIterator[RawDoc]:
        try:
            resp = await client.get(
                "https://api.github.com/search/repositories",
                params={"q": query, "per_page": max_results},
                headers={"Accept": "application/vnd.github+json"},
            )
            resp.raise_for_status()
            for item in resp.json().get("items", [])[:max_results]:
                content = "\n".join([
                    item.get("description") or "",
                    f"Stars: {item.get('stargazers_count', 0)}",
                    f"Language: {item.get('language') or ''}",
                    f"Repository: {item.get('full_name') or ''}",
                ]).strip()
                if not content:
                    continue
                yield RawDoc(
                    url=item.get("html_url", ""),
                    title=item.get("full_name", ""),
                    content=content,
                    source="github-search",
                    fetched_at=datetime.now(timezone.utc).isoformat(),
                )
        except Exception:
            return

    async def _summary_doc(self, client: httpx.AsyncClient, title: str) -> RawDoc | None:
        try:
            resp = await client.get(f"{self.SUMMARY_URL}/{quote(title.replace(' ', '_'))}")
            if resp.status_code != 200:
                return None
            data = resp.json()
            content = data.get("extract") or ""
            if not content:
                return None
            return RawDoc(
                url=data.get("content_urls", {}).get("desktop", {}).get("page", ""),
                title=data.get("title", title),
                content=content,
                source=self.name,
                fetched_at=datetime.now(timezone.utc).isoformat(),
            )
        except Exception:
            return None
