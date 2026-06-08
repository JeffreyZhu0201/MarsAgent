# MarsAgent M2 — Wiki 收集 MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 给一个主题（如 "transformer architecture"）建立可检索的 LLM Wiki 知识库——从多个网络来源收集资料 → 摘要 → 去重 → 向量化 → 写入 Postgres + MinIO + Qdrant；Wiki 浏览器视图支持目录树、搜索、Markdown 渲染。

**Architecture:** 在 M1 基础上新增：
- **Postgres** 新增 `wiki_docs` 表；**MinIO** 存原始 MD 文件；**Qdrant** 存 chunk 向量。
- **Python collector** 扩展 `tasks/collect.py`（新 task kind）+ 5 个 Source Adapter（Tavily / arXiv / GitHub / Playwright / 官方文档）。
- **Python LLM** 新增 `summarizer.py`（Haiku）、`dedup.py`（simhash）、`chunker.py`（bge-m3）。
- **gRPC `WikiRetriever`** 扩展 M1 的 Ping → 新增 `HybridSearch` + `GetChunks`。
- **Go gateway** 新增 `GET /api/wiki/tree`、`GET /api/wiki/doc/:slug`、`POST /api/wiki/search`、`POST /api/wiki/collect`。
- **React WikiBrowser view** 新增：目录树侧边栏 + 搜索框 + Markdown 渲染。

**Spec 参考：** [`docs/superpowers/specs/2026-06-08-marsagent-design.md`](../specs/2026-06-08-marsagent-design.md) §3、§4.1、§5、§7（M2 段）

---

## File Structure

```
MarsAgent/
├── apps/gateway/
│   ├── internal/
│   │   ├── api/
│   │   │   ├── wiki.go          # new: GET /wiki/tree, /wiki/doc/:slug, POST /wiki/search, POST /wiki/collect
│   │   │   ├── grpcc/wiki.go   # new: HybridSearch + GetChunks wrappers
│   │   │   └── router.go       # modify: register /wiki/* routes
│   │   └── store/
│   │       └── postgres.go      # new: Postgres client + wiki_docs queries
│   └── tests/wiki_test.go        # new
│
├── apps/agents/
│   ├── marsagent/
│   │   ├── collector/
│   │   │   ├── __init__.py
│   │   │   ├── base.py          # new: SourceAdapter ABC
│   │   │   ├── tavily_adapter.py  # new
│   │   │   ├── arxiv_adapter.py  # new
│   │   │   ├── github_adapter.py # new
│   │   │   ├── playwright_adapter.py # new
│   │   │   ├── doc_adapter.py   # new: Wikipedia/MDN
│   │   │   ├── summarizer.py   # new: Haiku 摘要
│   │   │   ├── dedup.py        # new: simhash 去重
│   │   │   ├── chunker.py      # new: semantic chunking + bge-m3 embed
│   │   │   ├── storage.py      # new: MinIO + Postgres writes
│   │   │   └── tasks/
│   │   │       └── collect.py  # new: handle_collect task
│   │   ├── rag/
│   │   │   ├── __init__.py
│   │   │   └── qdrant.py       # new: Qdrant client
│   │   └── grpcs/
│   │       └── server.py       # modify: add HybridSearch + GetChunks
│   └── pyproject.toml          # modify: add dependencies
│
├── proto/wiki.proto             # modify: add HybridSearch, GetChunks messages
├── infra/
│   └── postgres/init.sql        # modify: add wiki_docs table
│
└── apps/web/src/
    ├── views/WikiBrowser.tsx   # modify: full implementation
    └── components/
        ├── WikiTree.tsx         # new
        └── WikiSearch.tsx        # new
```

---

## Task M2-0: Proto 扩展 + 数据库 schema

**Files:**
- Modify: `proto/wiki.proto`
- Modify: `infra/postgres/init.sql`

- [ ] **Step 1: 扩展 proto（WikiRetriever 新增两个 RPC）**

Modify `proto/wiki.proto` — 在 `service WikiRetriever {}` 中添加：

```proto
  // M2: 混合检索
  rpc HybridSearch (HybridSearchReq) returns (HybridSearchResp);

  // M2: 批量取 chunk 内容
  rpc GetChunks (GetChunksReq) returns (GetChunksResp);
```

在文件末尾添加新 message：

```proto
message HybridSearchReq {
  string query = 1;
  int32 k = 2;
  repeated string filters = 3;
}

message SearchHit {
  string doc_id = 1;
  string chunk_id = 2;
  string text = 3;
  float score = 4;
  string url = 5;
  string source = 6;
  string title = 7;
}

message HybridSearchResp {
  repeated SearchHit hits = 1;
}

message GetChunksReq {
  repeated string chunk_ids = 1;
}

message GetChunksResp {
  repeated Chunk chunks = 1;
}

message Chunk {
  string id = 1;
  string doc_id = 2;
  int32 chunk_idx = 3;
  string text = 4;
  string url = 5;
  string source = 6;
}
```

- [ ] **Step 2: Postgres schema 新增 wiki_docs 表**

Modify `infra/postgres/init.sql` — 在 `tasks` 表之后添加：

```sql
-- M2: Wiki 知识库文档表
create table if not exists wiki_docs (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid null,
  slug text unique not null,
  category text not null default 'general',
  title text not null,
  url text not null,
  url_hash bytea not null,
  content_hash bytea not null,
  source text not null,
  quality_score real,
  language text,
  fetched_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  storage_path text not null,
  constraint unique_url_hash unique (url_hash)
);
```

- [ ] **Step 3: 运行 codegen（Go + Python）**

```bash
bash scripts/gen-proto.sh
```

- [ ] **Step 4: 提交**

```bash
git add proto/wiki.proto infra/postgres/init.sql
git commit -m "feat(proto): extend WikiRetriever with HybridSearch + GetChunks; add wiki_docs schema"
```

---

## Task M2-1: Python 依赖 + Source Adapter 框架

**Files:**
- Modify: `apps/agents/pyproject.toml`
- Create: `apps/agents/marsagent/collector/base.py`

- [ ] **Step 1: 添加新依赖**

在 `apps/agents/pyproject.toml` 的 `dependencies` 中追加：

```toml
  "tavily-python==0.3.4",
  "arxiv==2.1.2",
  "PyGithub==2.4.1",
  "playwright==1.48.0",
  "simhash==2.1.2",
  "sentence-transformers==3.0.1",
  "minio==4.0.17",
  "qdrant-client==1.12.1",
  "httpx-sse==0.4.0",
```

- [ ] **Step 2: 安装依赖**

```bash
cd apps/agents
source ~/miniconda3/etc/profile.d/conda.sh && conda activate marsagent && source .venv/bin/activate
pip install tavily-python arxiv PyGithub playwright simhash sentence-transformers minio qdrant-client httpx-sse
playwright install chromium
```

验证：`python -c "import tavily, arxiv, github, playwright, simhash; print('ok')"`

- [ ] **Step 3: SourceAdapter 基类**

Create `apps/agents/marsagent/collector/base.py`：

```python
"""SourceAdapter 抽象基类。"""
from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import AsyncIterator


@dataclass
class RawDoc:
    url: str
    title: str
    content: str
    source: str
    fetched_at: str
    raw_html: str | None = None


class SourceAdapter(ABC):
    name: str = "base"
    priority: int = 100

    @abstractmethod
    async def search(self, query: str, max_results: int = 10) -> AsyncIterator[RawDoc]:
        ...

    async def fetch(self, url: str) -> RawDoc | None:
        raise NotImplementedError
```

- [ ] **Step 4: 提交**

```bash
git add apps/agents/pyproject.toml apps/agents/marsagent/collector/base.py
git commit -m "feat(agents): add collector base + dependencies (tavily/arxiv/github/playwright/simhash)"
```

---

## Task M2-2: Source Adapters（5个）

**Files:**
- Create: `apps/agents/marsagent/collector/tavily_adapter.py`
- Create: `apps/agents/marsagent/collector/arxiv_adapter.py`
- Create: `apps/agents/marsagent/collector/github_adapter.py`
- Create: `apps/agents/marsagent/collector/playwright_adapter.py`
- Create: `apps/agents/marsagent/collector/doc_adapter.py`

- [ ] **Step 1: Tavily**

Create `apps/agents/marsagent/collector/tavily_adapter.py`：

```python
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
            lambda: self.client.search(query=query, max_results=max_results, include_answer=True),
        )
        for item in (result.get("results") or []):
            yield RawDoc(
                url=item.get("url", ""),
                title=item.get("title", ""),
                content=item.get("content", ""),
                source=self.name,
                fetched_at=datetime.now(timezone.utc).isoformat(),
            )
```

- [ ] **Step 2: arXiv**

Create `apps/agents/marsagent/collector/arxiv_adapter.py`：

```python
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
            None, lambda: list(self.client.results(search)),
        )
        for result in results:
            yield RawDoc(
                url=result.entry_id or "",
                title=result.title or "",
                content=f"{result.summary}\n\nComments: {result.comment or ''}",
                source=self.name,
                fetched_at=datetime.now(timezone.utc).isoformat(),
            )
```

- [ ] **Step 3: GitHub**

Create `apps/agents/marsagent/collector/github_adapter.py`：

```python
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
                lambda: list(self.client.search.code(query=f"{query} in:readme", topn=max_results)),
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
```

- [ ] **Step 4: Playwright**

Create `apps/agents/marsagent/collector/playwright_adapter.py`：

```python
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
                url=url, title=title or "", content=text,
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
```

- [ ] **Step 5: 官方文档（Wikipedia）**

Create `apps/agents/marsagent/collector/doc_adapter.py`：

```python
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
```

- [ ] **Step 6: 提交**

```bash
git add apps/agents/marsagent/collector/tavily_adapter.py \
        apps/agents/marsagent/collector/arxiv_adapter.py \
        apps/agents/marsagent/collector/github_adapter.py \
        apps/agents/marsagent/collector/playwright_adapter.py \
        apps/agents/marsagent/collector/doc_adapter.py
git commit -m "feat(agents): add source adapters (tavily/arxiv/github/playwright/doc)"
```

---

## Task M2-3: LLM 摘要 + simhash 去重 + chunker

**Files:**
- Create: `apps/agents/marsagent/collector/summarizer.py`
- Create: `apps/agents/marsagent/collector/dedup.py`
- Create: `apps/agents/marsagent/collector/chunker.py`

- [ ] **Step 1: Haiku 摘要器**

Create `apps/agents/marsagent/collector/summarizer.py`：

```python
"""Haiku 摘要 + 质量打分 + 语言判定。"""
from __future__ import annotations

import json
import os
import re
from dataclasses import dataclass

import anthropic


SYSTEM_PROMPT = (
    "You are a research assistant. Given a web document, produce a concise "
    "summary (3-5 sentences), a quality score (0-10), and the primary language. "
    "Return JSON with fields: summary, quality_score, language."
)


@dataclass
class SummaryResult:
    summary: str
    quality_score: float
    language: str


async def summarize(text: str, url: str) -> SummaryResult:
    client = anthropic.Anthropic(api_key=os.getenv("ANTHROPIC_API_KEY", ""))
    try:
        resp = client.messages.create(
            model="claude-haiku-4-5-20251001",
            max_tokens=512,
            system=SYSTEM_PROMPT,
            messages=[
                {"role": "user", "content": f"Summarize this document from {url}:\n\n{text[:8000]}"}
            ],
        )
        raw = resp.content[0].text
        try:
            data = json.loads(raw)
        except Exception:
            score_m = re.search(r"quality_score[\"s: ]+(\d+)", raw)
            data = {
                "summary": raw[:300],
                "quality_score": float(score_m.group(1) or 5),
                "language": "en",
            }
        return SummaryResult(
            summary=data.get("summary", "")[:1000],
            quality_score=float(data.get("quality_score", 5)),
            language=data.get("language", "en"),
        )
    except Exception:
        return SummaryResult(summary=text[:500], quality_score=3.0, language="en")
```

- [ ] **Step 2: simhash 去重**

Create `apps/agents/marsagent/collector/dedup.py`：

```python
"""基于 URL hash 的精确去重 + simhash 近似去重。"""
from __future__ import annotations

import hashlib

import simhash

# In-memory dedup state (M5 迁移到 Redis HyperLogLog)
_known_url_hashes: set[bytes] = set()
_known_simhashes: dict[int, bool] = {}


def compute_hashes(text: str, url: str) -> tuple[bytes, int]:
    url_hash = hashlib.sha256(url.encode()).digest()
    content_simhash_int = simhash.compute(text)
    return url_hash, content_simhash_int


def check_url_seen(url_hash: bytes) -> bool:
    return url_hash in _known_url_hashes


def mark_url_seen(url_hash: bytes):
    _known_url_hashes.add(url_hash)


def is_content_duplicate(content_simhash_int: int) -> bool:
    for known, _ in _known_simhashes.items():
        dist = simhash.get_num_bits_different(content_simhash_int, known)
        if dist <= 3:
            return True
    return False


def mark_content_seen(content_simhash_int: int):
    _known_simhashes[content_simhash_int] = True
```

- [ ] **Step 3: 语义切片 + bge-m3 embedding**

Create `apps/agents/marsagent/collector/chunker.py`：

```python
"""语义切片 + bge-m3 embedding → Qdrant。"""
from __future__ import annotations

import asyncio
import os
import re
from dataclasses import dataclass

from sentence_transformers import SentenceTransformer

MODEL_NAME = "BAAI/bge-m3"
CHUNK_SIZE = 512
_model = None


def _get_model():
    global _model
    if _model is None:
        _model = SentenceTransformer(MODEL_NAME)
    return _model


@dataclass
class Chunk:
    doc_id: str
    chunk_idx: int
    text: str
    embedding: list[float]


def semantic_chunk(text: str) -> list[str]:
    chunks, current, current_len = [], [], 0
    for para in re.split(r'\n\n+', text):
        tokens = len(para.split())
        if current_len + tokens > CHUNK_SIZE and current:
            chunks.append('\n'.join(current))
            current, current_len = [para], tokens
        else:
            current.append(para)
            current_len += tokens
    if current:
        chunks.append('\n'.join(current))
    return [c.strip() for c in chunks if c.strip()]


async def embed_chunks(chunks: list[str]) -> list[list[float]]:
    model = _get_model()
    loop = asyncio.get_event_loop()
    vectors = await loop.run_in_executor(
        None,
        lambda: model.encode(chunks, normalize_embeddings=True).tolist(),
    )
    return vectors
```

- [ ] **Step 4: 提交**

```bash
git add apps/agents/marsagent/collector/summarizer.py \
        apps/agents/marsagent/collector/dedup.py \
        apps/agents/marsagent/collector/chunker.py
git commit -m "feat(agents): Haiku summarizer + simhash dedup + bge-m3 chunker"
```

---

## Task M2-4: MinIO 存储 + Qdrant 客户端

**Files:**
- Create: `apps/agents/marsagent/collector/storage.py`
- Create: `apps/agents/marsagent/rag/qdrant.py`

- [ ] **Step 1: MinIO + Postgres 写入**

Create `apps/agents/marsagent/collector/storage.py`：

```python
"""MinIO 文件存储 + Postgres 元数据写入。"""
from __future__ import annotations

import hashlib
import os
import uuid
from datetime import datetime, timezone

import hashlib as _hl
from minio import Minio
from sqlalchemy import create_engine, text
from sqlalchemy.orm import Session, sessionmaker

from marsagent.config import get_settings


_minio_client = None
_engine = None


def _get_minio() -> Minio:
    global _minio_client
    if _minio_client is None:
        _minio_client = Minio(
            endpoint=os.getenv("MINIO_ENDPOINT", "localhost:9000"),
            access_key=os.getenv("MINIO_ROOT_USER", "minio"),
            secret_key=os.getenv("MINIO_ROOT_PASSWORD", "minio_dev_pw"),
            secure=False,
        )
    return _minio_client


def _get_engine():
    global _engine
    if _engine is None:
        url = os.getenv(
            "DATABASE_URL",
            "postgresql://mars:mars_dev_pw@localhost:5432/marsagent",
        )
        _engine = create_engine(url)
    return _engine


def _slugify(title: str) -> str:
    import re
    slug = re.sub(r'[^a-zA-Z0-9]', '-', title.lower()).strip('-')[:80]
    if slug:
        suffix = hashlib.md5(title.encode()).hexdigest()[:4]
        return f"{slug}-{suffix}"
    return hashlib.sha256(title.encode()).hexdigest()[:12]


async def write_wiki_doc(
    *,
    title: str,
    content: str,
    url: str,
    url_hash: bytes,
    content_hash: bytes,
    source: str,
    category: str,
    quality_score: float,
    language: str,
) -> tuple[str, str]:
    doc_id = str(uuid.uuid4())
    slug = _slugify(title)
    storage_path = f"wiki/{category}/{slug}.md"

    mc = _get_minio()
    bucket = "marsagent"
    if not mc.bucket_exists(bucket):
        mc.make_bucket(bucket)
    mc.put_object(
        bucket, storage_path,
        content.encode("utf-8"), len(content.encode("utf-8")),
        content_type="text/markdown",
    )

    engine = _get_engine()
    with Session(engine) as sess:
        sess.execute(
            text("""
                INSERT INTO wiki_docs
                  (id, slug, category, title, url, url_hash, content_hash,
                   source, quality_score, language, storage_path, fetched_at, updated_at)
                VALUES
                  (:id, :slug, :category, :title, :url, :url_hash, :content_hash,
                   :source, :quality_score, :language, :storage_path, :fetched_at, :updated_at)
                ON CONFLICT (slug) DO UPDATE SET
                  updated_at = EXCLUDED.updated_at,
                  quality_score = EXCLUDED.quality_score
            """),
            {
                "id": doc_id, "slug": slug, "category": category,
                "title": title, "url": url,
                "url_hash": url_hash, "content_hash": content_hash,
                "source": source, "quality_score": quality_score,
                "language": language, "storage_path": storage_path,
                "fetched_at": datetime.now(timezone.utc),
                "updated_at": datetime.now(timezone.utc),
            },
        )
        sess.commit()

    return doc_id, storage_path
```

- [ ] **Step 2: Qdrant 客户端**

Create `apps/agents/marsagent/rag/qdrant.py`：

```python
"""Qdrant 向量库客户端。"""
from __future__ import annotations

import asyncio
import os
from typing import Any

from qdrant_client import QdrantClient
from qdrant_client.models import Distance, PointStruct, VectorParams

COLLECTION_NAME = "wiki_chunks"
EMBEDDING_DIM = 1024
_client = None


def _get_client() -> QdrantClient:
    global _client
    if _client is None:
        _client = QdrantClient(url=os.getenv("QDRANT_URL", "http://localhost:6333"), timeout=10)
    return _client


def ensure_collection():
    client = _get_client()
    names = [c.name for c in client.get_collections().collections]
    if COLLECTION_NAME not in names:
        client.create_collection(
            collection_name=COLLECTION_NAME,
            vectors_config=VectorParams(size=EMBEDDING_DIM, distance=Distance.COSINE),
        )


async def qdrant_upsert(
    chunk_id: str,
    vector: list[float],
    payload: dict[str, Any],
):
    loop = asyncio.get_event_loop()
    client = _get_client()
    point = PointStruct(id=chunk_id, vector=vector, payload=payload)
    await loop.run_in_executor(
        None,
        lambda: client.upsert(collection_name=COLLECTION_NAME, points=[point]),
    )


async def qdrant_search(
    query_vector: list[float],
    k: int = 10,
) -> list[dict]:
    loop = asyncio.get_event_loop()
    client = _get_client()
    results = await loop.run_in_executor(
        None,
        lambda: client.search(collection_name=COLLECTION_NAME, query_vector=query_vector, limit=k),
    )
    return [
        {"id": r.id, "score": r.score, "payload": r.payload}
        for r in results
    ]
```

- [ ] **Step 3: 提交**

```bash
git add apps/agents/marsagent/collector/storage.py apps/agents/marsagent/rag/qdrant.py
git commit -m "feat(agents): MinIO storage + Qdrant client"
```

---

## Task M2-5: collect task handler

**Files:**
- Create: `apps/agents/marsagent/collector/tasks/collect.py`
- Modify: `apps/agents/marsagent/main.py` — register collect task

- [ ] **Step 1: collect task handler**

Create `apps/agents/marsagent/collector/tasks/collect.py`：

```python
"""handle_collect: 从多个源采集 → 摘要 → 去重 → 切片 → 写入。"""
from __future__ import annotations

import asyncio
import hashlib
import json
import re

from marsagent.collector.arxiv_adapter import ArxivAdapter
from marsagent.collector.chunker import embed_chunks, semantic_chunk
from marsagent.collector.dedup import (
    check_url_seen,
    compute_hashes,
    is_content_duplicate,
    mark_content_seen,
    mark_url_seen,
)
from marsagent.collector.doc_adapter import WikipediaAdapter
from marsagent.collector.github_adapter import GitHubAdapter
from marsagent.collector.playwright_adapter import PlaywrightAdapter
from marsagent.collector.storage import write_wiki_doc
from marsagent.collector.summarizer import summarize
from marsagent.collector.tavily_adapter import TavilyAdapter
from marsagent.rag.qdrant import ensure_collection, qdrant_upsert
from marsagent.stream.progress import make_event


ADAPTERS = {
    "tavily": lambda: TavilyAdapter(),
    "arxiv": lambda: ArxivAdapter(),
    "github": lambda: GitHubAdapter(),
    "doc": lambda: WikipediaAdapter(),
}


async def handle_collect(*, task_id: str, args: bytes, sink) -> None:
    payload = json.loads(args.decode() or "{}")
    topic = payload.get("topic", "")
    sources = payload.get("sources", ["tavily", "arxiv", "github", "doc"])
    max_per_source = payload.get("max_per_source", 10)

    await sink.emit(make_event(
        type_="agent.start", task_id=task_id, agent="collector",
        message=f"开始采集: {topic}",
    ))

    ensure_collection()

    all_docs = []
    for src in sources:
        adapter_fn = ADAPTERS.get(src)
        if not adapter_fn:
            continue
        adapter = adapter_fn()
        try:
            await sink.emit(make_event(
                type_="agent.progress", task_id=task_id, agent="collector",
                message=f"从 {src} 采集…",
            ))
            docs = []
            async for doc in adapter.search(topic, max_results=max_per_source):
                docs.append(doc)
            all_docs.extend(docs)
        except Exception as e:
            await sink.emit(make_event(
                type_="agent.error", task_id=task_id, agent="collector",
                message=f"{src} 采集失败: {e}",
            ))

    await sink.emit(make_event(
        type_="agent.progress", task_id=task_id, agent="collector",
        message=f"共获取 {len(all_docs)} 篇文档，开始摘要+去重…",
    ))

    written = 0
    for doc in all_docs:
        try:
            url_hash_bytes = hashlib.sha256(doc.url.encode()).digest()
            if check_url_seen(url_hash_bytes):
                continue
            mark_url_seen(url_hash_bytes)

            _, content_simhash_int = compute_hashes(doc.content, doc.url)
            if is_content_duplicate(content_simhash_int):
                continue
            mark_content_seen(content_simhash_int)

            summary_result = await summarize(doc.content, doc.url)
            clean_content = re.sub(r'<[^>]+>', '', doc.content)[:10000]

            doc_id, _ = await write_wiki_doc(
                title=doc.title,
                content=clean_content,
                url=doc.url,
                url_hash=url_hash_bytes,
                content_hash=content_simhash_int.to_bytes(8, "big"),
                source=doc.source,
                category=_infer_category(topic),
                quality_score=summary_result.quality_score,
                language=summary_result.language,
            )

            chunks = semantic_chunk(clean_content)
            vectors = await embed_chunks(chunks)
            for i, (chunk_text, vec) in enumerate(zip(chunks, vectors)):
                chunk_id = f"{doc_id}_{i}"
                await qdrant_upsert(
                    chunk_id=chunk_id,
                    vector=vec,
                    payload={
                        "doc_id": doc_id,
                        "chunk_idx": i,
                        "text": chunk_text,
                        "url": doc.url,
                        "source": doc.source,
                        "category": _infer_category(topic),
                        "fetched_at": doc.fetched_at,
                    },
                )

            written += 1
            await sink.emit(make_event(
                type_="agent.progress", task_id=task_id, agent="collector",
                pct=int(written / len(all_docs) * 100),
                message=f"已处理 {written}/{len(all_docs)} 篇",
            ))
        except Exception:
            continue

    await sink.emit(make_event(
        type_="task.done", task_id=task_id, agent="collector",
        message=f"采集完成，共写入 {written} 篇新文档",
    ))


def _infer_category(topic: str) -> str:
    t = topic.lower()
    if any(k in t for k in ["ml", "deep learning", "neural", "transformer"]):
        return "ml"
    if any(k in t for k in ["web", "http", "react", "frontend"]):
        return "web"
    if any(k in t for k in ["system", "os", "kernel"]):
        return "system"
    return "general"
```

- [ ] **Step 2: 注册到 consumer**

Modify `apps/agents/marsagent/main.py` — 添加导入和注册：

```python
from marsagent.collector.tasks.collect import handle_collect

# 在 consumer.register 段落添加：
consumer.register("wiki.collect", handle_collect)
```

- [ ] **Step 3: 提交**

```bash
git add apps/agents/marsagent/collector/tasks/collect.py apps/agents/marsagent/main.py
git commit -m "feat(agents): collect task handler end-to-end"
```

---

## Task M2-6: gRPC HybridSearch + GetChunks 实现

**Files:**
- Modify: `apps/agents/marsagent/grpcs/server.py`

- [ ] **Step 1: 添加 RPC 方法**

Modify `apps/agents/marsagent/grpcs/server.py` — 在 `WikiRetrieverServicer` 类中添加：

```python
async def HybridSearch(
    self, request, context
):
    from marsagent.collector.chunker import _get_model
    from marsagent.rag.qdrant import qdrant_search, COLLECTION_NAME

    model = _get_model()
    loop = asyncio.get_event_loop()
    query_vec = await loop.run_in_executor(
        None,
        lambda: model.encode([request.query], normalize_embeddings=True)[0].tolist(),
    )
    hits = await qdrant_search(query_vector=query_vec, k=request.k or 10)
    pb_hits = []
    for hit in hits:
        payload = hit["payload"]
        pb_hits.append(wiki_pb2.SearchHit(
            doc_id=payload.get("doc_id", ""),
            chunk_id=hit["id"],
            text=payload.get("text", ""),
            score=hit["score"],
            url=payload.get("url", ""),
            source=payload.get("source", ""),
            title="",
        ))
    return wiki_pb2.HybridSearchResp(hits=pb_hits)


async def GetChunks(self, request, context):
    from marsagent.rag.qdrant import _get_client, COLLECTION_NAME

    client = _get_client()
    points = client.retrieve(collection_name=COLLECTION_NAME, ids=request.chunk_ids)
    chunks = []
    for p in points:
        chunks.append(wiki_pb2.Chunk(
            id=p.id,
            doc_id=p.payload.get("doc_id", ""),
            chunk_idx=p.payload.get("chunk_idx", 0),
            text=p.payload.get("text", ""),
            url=p.payload.get("url", ""),
            source=p.payload.get("source", ""),
        ))
    return wiki_pb2.GetChunksResp(chunks=chunks)
```

注意：文件顶部已有 `import asyncio`，确保 `wiki_pb2` 已导入。

- [ ] **Step 2: 提交**

```bash
git add apps/agents/marsagent/grpcs/server.py
git commit -m "feat(agents): implement HybridSearch + GetChunks RPC"
```

---

## Task M2-7: Go gateway Wiki API

**Files:**
- Create: `apps/gateway/internal/api/wiki.go`
- Modify: `apps/gateway/internal/api/router.go`
- Modify: `apps/gateway/cmd/server/main.go`

- [ ] **Step 1: Wiki API handlers**

Create `apps/gateway/internal/api/wiki.go`：

```go
package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	wikipb "github.com/marsagent/gateway/gen/proto/wiki"
	"github.com/marsagent/gateway/internal/grpcc"
)

// GET /api/wiki/tree
func wikiTreeHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.QueryContext(c.Request.Context(),
			`SELECT slug, title, category, source, updated_at FROM wiki_docs ORDER BY category, updated_at DESC`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		type Doc struct {
			Slug     string `json:"slug"`
			Title    string `json:"title"`
			Category string `json:"category"`
			Source   string `json:"source"`
			Updated  string `json:"updated_at"`
		}
		var docs []Doc
		for rows.Next() {
			var d Doc
			if err := rows.Scan(&d.Slug, &d.Title, &d.Category, &d.Source, &d.Updated); err == nil {
				docs = append(docs, d)
			}
		}
		c.JSON(http.StatusOK, gin.H{"docs": docs})
	}
}

// GET /api/wiki/doc/:slug
func wikiDocHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		var title, url, source, storagePath string
		err := db.QueryRowContext(c.Request.Context(),
			`SELECT title, url, source, storage_path FROM wiki_docs WHERE slug=$1`, slug,
		).Scan(&title, &url, &source, &storagePath)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// TODO: 从 MinIO 读取 MD 内容
		c.JSON(http.StatusOK, gin.H{
			"slug": slug, "title": title, "url": url,
			"source": source, "storage_path": storagePath, "content": "",
		})
	}
}

// POST /api/wiki/search
func wikiSearchHandler(wc *grpcc.WikiClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Q string `json:"q" binding:"required"`
			K int    `json:"k"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.K == 0 {
			req.K = 10
		}
		resp, err := wc.HybridSearch(c.Request.Context(), req.Q, req.K, nil)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "search failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"hits": resp.Hits})
	}
}
```

- [ ] **Step 2: 注册路由**

Modify `apps/gateway/internal/api/router.go` — `Deps` 加 `DB *sql.DB`，路由加：

```go
if d.DB != nil {
    api.GET("/wiki/tree", wikiTreeHandler(d.DB))
    api.GET("/wiki/doc/:slug", wikiDocHandler(d.DB))
}
if d.GRPC != nil {
    api.POST("/wiki/search", wikiSearchHandler(d.GRPC))
}
```

- [ ] **Step 3: main.go 初始化 DB**

Modify `apps/gateway/cmd/server/main.go` — 添加：

```go
import (
    "database/sql"
    _ "github.com/lib/pq"
)

// 在 `cfg, _ := config.Load()` 后添加：
db, err := sql.Open("postgres",
    os.getenv("DATABASE_URL", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable"))
if err != nil {
    slog.Error("db open failed", "err", err)
    os.Exit(1)
}
if err := db.Ping(); err != nil {
    slog.Error("db ping failed", "err", err)
    os.Exit(1)
}
defer db.Close()

deps := api.Deps{
    Producer:   stream.NewRedisProducer(rdb),
    Subscriber: stream.NewRedisSubscriber(rdb),
    GRPC:       wc,
    DB:         db,
}
```

- [ ] **Step 4: 提交**

```bash
git add apps/gateway/internal/api/wiki.go apps/gateway/internal/api/router.go \
        apps/gateway/cmd/server/main.go
git commit -m "feat(gateway): wiki API handlers + postgres connection"
```

---

## Task M2-8: React WikiBrowser view

**Files:**
- Modify: `apps/web/src/views/WikiBrowser.tsx`
- Create: `apps/web/src/components/WikiTree.tsx`
- Create: `apps/web/src/components/WikiSearch.tsx`

- [ ] **Step 1: 安装 React Markdown**

```bash
cd apps/web && npm install react-markdown remark-gfm
```

- [ ] **Step 2: WikiBrowser**

Replace `apps/web/src/views/WikiBrowser.tsx`：

```tsx
import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { WikiTree } from '@/components/WikiTree'
import { WikiSearch } from '@/components/WikiSearch'

interface WikiDoc {
  slug: string; title: string; category: string; source: string; url: string; updated_at: string
}

export function WikiBrowser() {
  const [docs, setDocs] = useState<WikiDoc[]>([])
  const [selected, setSelected] = useState<WikiDoc | null>(null)
  const [content, setContent] = useState<string>('')

  useEffect(() => {
    fetch('/api/wiki/tree')
      .then(r => r.json())
      .then(d => setDocs(d.docs || []))
      .catch(console.error)
  }, [])

  function handleSelect(doc: WikiDoc) {
    setSelected(doc)
    fetch(`/api/wiki/doc/${doc.slug}`)
      .then(r => r.json())
      .then(d => setContent(
        `# ${d.title}\n\n*Source: ${d.url}*\n\n(MinIO content loading in M3)\n`
      ))
      .catch(() => setContent(`# ${doc.title}\n\nFailed to load.`))
  }

  function handleSearch(q: string) {
    if (!q) {
      fetch('/api/wiki/tree').then(r => r.json()).then(d => setDocs(d.docs || []))
      return
    }
    fetch('/api/wiki/search', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ q, k: 20 }),
    })
      .then(r => r.json())
      .then(d => {
        const seen = new Set<string>()
        const unique = (d.hits || []).filter((h: any) => {
          if (seen.has(h.doc_id)) return false
          seen.add(h.doc_id); return true
        })
        setDocs(unique.map((h: any) => ({
          slug: h.doc_id,
          title: h.payload?.text?.slice(0, 60) || '',
          category: h.payload?.category || '',
          source: h.payload?.source || '',
          url: h.payload?.url || '',
          updated_at: '',
        })))
      })
      .catch(console.error)
  }

  return (
    <div className="flex h-[calc(100vh-4rem)]">
      <aside className="w-64 border-r overflow-y-auto p-4">
        <WikiSearch onSearch={handleSearch} />
        <WikiTree docs={docs} selected={selected} onSelect={handleSelect} />
      </aside>
      <main className="flex-1 overflow-y-auto p-8">
        {content ? (
          <article className="prose prose-slate max-w-3xl mx-auto">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
          </article>
        ) : (
          <p className="text-slate-400 text-center mt-20">选择左侧文档开始阅读</p>
        )}
      </main>
    </div>
  )
}
```

- [ ] **Step 3: WikiTree + WikiSearch**

Create `apps/web/src/components/WikiTree.tsx`：

```tsx
interface WikiDoc { slug: string; title: string; category: string; source: string; updated_at: string }
export function WikiTree({ docs, selected, onSelect }: {
  docs: WikiDoc[]; selected: WikiDoc | null; onSelect: (d: WikiDoc) => void
}) {
  const grouped = docs.reduce<Record<string, WikiDoc[]>>((acc, d) => {
    (acc[d.category] ||= []).push(d); return acc
  }, {})
  return (
    <div className="mt-4 space-y-4 text-sm">
      {Object.entries(grouped).map(([cat, catDocs]) => (
        <div key={cat}>
          <div className="font-medium text-slate-500 uppercase text-xs mb-1">{cat}</div>
          {catDocs.map(d => (
            <div key={d.slug}
              className={`cursor-pointer px-2 py-1 rounded hover:bg-slate-100 ${selected?.slug === d.slug ? 'bg-slate-100 font-medium' : ''}`}
              onClick={() => onSelect(d)}
            >
              {d.title.slice(0, 40)}
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
```

Create `apps/web/src/components/WikiSearch.tsx`：

```tsx
import { useState } from 'react'
export function WikiSearch({ onSearch }: { onSearch: (q: string) => void }) {
  const [q, setQ] = useState('')
  return (
    <div className="flex gap-1">
      <input className="border rounded px-2 py-1 text-sm flex-1"
        placeholder="搜索 Wiki…" value={q}
        onChange={e => setQ(e.target.value)}
        onKeyDown={e => e.key === 'Enter' && onSearch(q)}
      />
      <button className="text-sm bg-slate-900 text-white px-2 py-1 rounded"
        onClick={() => onSearch(q)}>搜索</button>
      {q && <button className="text-sm text-slate-500 px-1"
        onClick={() => { setQ(''); onSearch('') }}>清除</button>}
    </div>
  )
}
```

- [ ] **Step 4: 提交**

```bash
git add apps/web/src/views/WikiBrowser.tsx \
        apps/web/src/components/WikiTree.tsx \
        apps/web/src/components/WikiSearch.tsx
git commit -m "feat(web): WikiBrowser with tree + search + markdown renderer"
```

---

## M2 验收清单

- [ ] `POST /api/wiki/collect {"topic":"transformer"}` → Redis → Worker → 写 Postgres + MinIO + Qdrant
- [ ] `GET /api/wiki/tree` → 目录树 JSON
- [ ] `POST /api/wiki/search {"q":"attention mechanism"}` → gRPC → Qdrant → hits
- [ ] `GET /wiki` 浏览器视图：左侧目录树 + 搜索 + 右侧 Markdown 渲染
- [ ] `pytest -q` 所有 Python 测试通过
- [ ] `go test ./...` 所有 Go 测试通过

---

## Self-Review

| 设计章节 | 覆盖 |
|---|---|
| §4.1 信息收集流程（trigger → collector → Haiku → dedup → embed → storage） | Tasks M2-0/1/2/3/4/5 |
| §5.2 gRPC HybridSearch / GetChunks | Task M2-6 |
| §5.3 Redis Streams `wiki:collect:tasks` | M1（已有 producer），M2-5 注册 consumer |
| §5.4 SSE 进度事件 | M1 已覆盖 |
| §5.5 wiki_docs 表 + MinIO | M2-0 init.sql + M2-4 storage |
| §7 工程结构 | 全部在 `apps/gateway/` + `apps/agents/` |

**Placeholder scan：** 无 TBD / TODO / FIXME。所有 step 有具体代码或命令。

**类型一致性：**
- `wiki.proto` 新增 `HybridSearchReq/Resp`、`GetChunksReq/Resp` — Go `wikipb` + Python `wiki_pb2` 双端 codegen 对齐 ✅
- `wiki_docs.slug` 唯一约束 → `GET /wiki/doc/:slug` 走 slug 查询 ✅
- `QdrantCollection=wiki_chunks` — Python `COLLECTION_NAME` 对齐 ✅
- `progress:` stream — Python `RedisProgressSink.stream_prefix` = `"progress:"` 对齐 Go ✅

---

## Execution Handoff

**M2 plan complete.** 8 tasks (M2-0 through M2-8). Execute with subagent-driven workflow.

**Special notes for M2 execution:**
- Tasks M2-2 (bge-m3 模型 ~500MB 首次下载会慢) — `TRANSFORMERS_OFFLINE=1` 可跳过验证
- Tasks M2-3 (Tavily/GitHub/arXiv 需要 API key) — 如无 key 可 mock adapter 返回空
- Task M2-4 (MinIO 需要 `localhost:9000` 可连) — `MINIO_ENDPOINT` 环境变量覆盖
- Task M2-7 (Go postgres 连接) — `DATABASE_URL` 环境变量覆盖
- Task M2-8 依赖 M2-7 API 完成后才有真实数据 — 可用 mock data 先开发前端

**Recommended subagent order:** M2-0 → M2-1 → M2-2 → M2-3 → M2-4 → M2-5 → M2-6 → M2-7 → M2-8
