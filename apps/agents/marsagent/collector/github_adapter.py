"""GitHub 适配器。"""
from __future__ import annotations

import asyncio
import os
from datetime import datetime, timezone
from typing import AsyncIterator

from github import Github
from github.GithubException import RateLimitExceededException

from .base import RawDoc, SourceAdapter


class GitHubAdapter(SourceAdapter):
    name = "github"
    priority = 30

    def __init__(self) -> None:
        token = os.getenv("GITHUB_TOKEN", "")
        self.client = Github(token or None)

    async def search(self, query: str, max_results: int = 10) -> AsyncIterator[RawDoc]:
        loop = asyncio.get_event_loop()
        try:
            result = await loop.run_in_executor(
                None,
                lambda: list(
                    self.client.search.code(
                        query=f"{query} in:readme", topn=max_results
                    )
                ),
            )
            for item in result:
                try:
                    repo = item.repository
                    readme = await loop.run_in_executor(
                        None,
                        lambda: repo.get_readme().decoded_content.decode()[:5000],
                    )
                    yield RawDoc(
                        url=item.html_url,
                        title=f"{repo.full_name}/{item.path}",
                        content=readme,
                        source=self.name,
                        fetched_at=datetime.now(timezone.utc).isoformat(),
                    )
                except Exception:
                    continue
        except RateLimitExceededException:
            pass
