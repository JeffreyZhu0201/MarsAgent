"""handle_collect: 从多个源采集 → 摘要 → 去重 → 切片 → 写入。"""
from __future__ import annotations

import asyncio
import hashlib
import json
import re
import uuid

from marsagent.collector.arxiv_adapter import ArxivAdapter
from marsagent.collector.base import SourceAdapter
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


ADAPTERS: dict[str, type[SourceAdapter]] = {
    "tavily": TavilyAdapter,
    "arxiv": ArxivAdapter,
    "github": GitHubAdapter,
    "doc": WikipediaAdapter,
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
        adapter_cls = ADAPTERS.get(src)
        if not adapter_cls:
            continue
        try:
            adapter = adapter_cls()
            await sink.emit(make_event(
                type_="agent.progress", task_id=task_id, agent="collector",
                message=f"从 {src} 采集…",
            ))
            docs = []
            async for doc in adapter.search(topic, max_results=max_per_source):
                docs.append(doc)
            all_docs.extend(docs)
            await sink.emit(make_event(
                type_="agent.progress", task_id=task_id, agent="collector",
                message=f"{src} 新发现 {len(docs)} 条",
                extra={
                    "stage": "discover",
                    "source": src,
                    "docs": [
                        {"title": d.title, "url": d.url, "source": d.source}
                        for d in docs[:20]
                    ],
                },
            ))
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
                await _emit_skip(sink, task_id, doc, "url_seen")
                continue

            _, content_simhash_int = compute_hashes(doc.content, doc.url)
            if is_content_duplicate(content_simhash_int):
                await _emit_skip(sink, task_id, doc, "content_duplicate")
                continue

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
                chunk_id = str(uuid.uuid5(uuid.UUID(doc_id), str(i)))
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

            mark_url_seen(url_hash_bytes)
            mark_content_seen(content_simhash_int)

            written += 1
            await sink.emit(make_event(
                type_="agent.progress", task_id=task_id, agent="collector",
                pct=int(written / max(len(all_docs), 1) * 100),
                message=f"已处理 {written}/{len(all_docs)} 篇",
                extra={
                    "stage": "write_wiki",
                    "doc": {"doc_id": doc_id, "title": doc.title, "url": doc.url, "source": doc.source},
                },
            ))
        except Exception as e:
            await sink.emit(make_event(
                type_="agent.error", task_id=task_id, agent="collector",
                message=f"处理失败: {doc.title or doc.url}: {e}",
                extra={
                    "stage": "process_error",
                    "doc": {"title": doc.title, "url": doc.url, "source": doc.source},
                    "error": str(e),
                },
            ))
            continue

    await sink.emit(make_event(
        type_="task.done", task_id=task_id, agent="collector",
        message=f"采集完成，共写入 {written} 篇新文档",
    ))


async def _emit_skip(sink, task_id: str, doc, reason: str) -> None:
    await sink.emit(make_event(
        type_="agent.progress", task_id=task_id, agent="collector",
        message=f"跳过: {doc.title or doc.url} ({reason})",
        extra={
            "stage": "skip",
            "reason": reason,
            "doc": {"title": doc.title, "url": doc.url, "source": doc.source},
        },
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
