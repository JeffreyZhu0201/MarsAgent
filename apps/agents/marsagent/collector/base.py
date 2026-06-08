"""SourceAdapter 抽象基类。"""
from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import AsyncIterator


@dataclass
class RawDoc:
    """归一化的原始文档结构。"""
    url: str
    title: str
    content: str
    source: str
    fetched_at: str
    raw_html: str | None = None


class SourceAdapter(ABC):
    """采集器基类。"""
    name: str = "base"
    priority: int = 100

    @abstractmethod
    async def search(self, query: str, max_results: int = 10) -> AsyncIterator[RawDoc]:
        """给定查询词，返回一批文档。"""
        ...

    async def fetch(self, url: str) -> RawDoc | None:
        """给定 URL，抓取单个页面。可选实现。"""
        raise NotImplementedError
