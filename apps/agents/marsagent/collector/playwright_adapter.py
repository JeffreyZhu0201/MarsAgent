"""Playwright 适配器（JS 渲染页面）。"""
from __future__ import annotations

import asyncio
import re
from datetime import datetime, timezone
from typing import AsyncIterator

from playwright.async_api import async_playwright

from .base import RawDoc, SourceAdapter


class PlaywrightAdapter(SourceAdapter):
    name = "playwright"
    priority = 50
    _playwright = None
    _browser = None

    async def _ensure_browser(self):
        if self._browser is None:
            self._playwright = await async_playwright().start()
            self._browser = await self._playwright.chromium.launch(headless=True)

    async def fetch(self, url: str) -> RawDoc | None:
        await self._ensure_browser()
        page = await self._browser.new_page()
        try:
            await page.goto(url, wait_until="networkidle", timeout=15000)
            html = await page.content()
            title = await page.title()
            text = re.sub(r'<[^>]+>', '', html)[:8000]
            return RawDoc(
                url=url,
                title=title or "",
                content=text,
                source=self.name,
                fetched_at=datetime.now(timezone.utc).isoformat(),
                raw_html=html,
            )
        except Exception:
            return None
        finally:
            await page.close()

    async def search(self, query: str, max_results: int = 10) -> AsyncIterator[RawDoc]:
        # Playwright 不做搜索；由 collect task 直接调用 fetch(url)
        pass

    async def close(self):
        if self._browser:
            await self._browser.close()
        if self._playwright:
            await self._playwright.stop()
