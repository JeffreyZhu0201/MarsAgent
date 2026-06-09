"""Planner 节点 — 使用 Opus 生成课程大纲。"""
from __future__ import annotations

import json
import anthropic

from marsagent.config import get_settings
from marsagent.llm import extract_thinking, model_for, response_text
from marsagent.stream.progress import make_event

from .state import Chapter, CourseState
from .prompts import PLANNER_SYSTEM, PLANNER_USER


async def _build_wiki_context(topic: str, rag_top_k: int) -> str:
    """Query Qdrant for top-k wiki chunks relevant to the topic."""
    try:
        from marsagent.collector.chunker import embed_chunks
        from marsagent.rag.qdrant import qdrant_search

        query_vecs = await embed_chunks([topic])
        hits = await qdrant_search(query_vector=query_vecs[0], k=rag_top_k)
        if not hits:
            return "（Wiki 知识库尚无相关文档）"

        parts = []
        for hit in hits:
            payload = hit.get("payload", {})
            text = payload.get("text", "")
            url = payload.get("url", "")
            source = payload.get("source", "")
            if text:
                src = f"[{source}]({url})" if url else source
                parts.append(f"--- 来源: {src} ---\n{text}")
        if not parts:
            return "（Wiki 知识库尚无相关文档）"
        return "\n\n".join(parts)
    except Exception:
        return "（Wiki 知识库查询失败，使用 LLM 自身知识）"


async def planner_node(state: CourseState, *, client: anthropic.Anthropic, rag_top_k: int = 20) -> CourseState:
    """Planner: 查 Wiki → 生成大纲。"""
    # Emit thinking-start marker
    if state.sink:
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="planner",
            message="Planner 正在检索 Wiki 知识库...",
        ))

    # Real RAG lookup
    wiki_context = await _build_wiki_context(state.topic, rag_top_k)

    if state.sink:
        hit_count = wiki_context.count("--- 来源:")
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="planner",
            message=f"Wiki 检索到 {hit_count} 个相关片段，开始规划课程结构...",
        ))

    user_prompt = PLANNER_USER.format(
        topic=state.topic,
        audience=state.audience,
        depth=state.depth,
        wiki_context=wiki_context,
    )

    resp = client.messages.create(
        model=model_for("opus"),
        max_tokens=4096,
        system=PLANNER_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )

    # Emit the raw thinking content
    thinking = extract_thinking(resp)
    if thinking and state.sink:
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="planner",
            message=f"Planner 推理过程:\n{thinking[:2000]}",
        ))

    raw = response_text(resp)
    start = raw.find("{")
    end = raw.rfind("}") + 1
    result = json.loads(raw[start:end])

    max_chapters = max(1, get_settings().builder_max_chapters)
    outline_data = result.get("outline", [])[:max_chapters]
    chapters = [
        Chapter(
            ch_id=ch["ch_id"],
            title=ch["title"],
            objectives=ch.get("objectives", []),
            prereqs=ch.get("prereqs", []),
            est_min=ch.get("est_min", 30),
            bloom_level=ch.get("bloom_level", "understand"),
            key_concepts=ch.get("key_concepts", []),
        )
        for ch in outline_data
    ]
    state.outline = chapters
    state.current_agent = "planner"
    state.pct = 10
    return state