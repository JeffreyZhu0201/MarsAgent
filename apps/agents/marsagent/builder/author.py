"""Author 节点 — Sonnet 生成章节正文。"""
from __future__ import annotations

from marsagent.llm import extract_thinking, model_for, response_text
from marsagent.stream.progress import make_event

from .state import Chapter, CourseState
from .prompts import AUTHOR_SYSTEM, AUTHOR_USER


async def _build_author_context(ch: Chapter, k: int = 5) -> str:
    """Query Qdrant for top-k wiki chunks relevant to a chapter."""
    try:
        from marsagent.collector.chunker import embed_chunks
        from marsagent.rag.qdrant import qdrant_search

        query = f"{ch.title} {' '.join(ch.key_concepts)}"
        query_vecs = await embed_chunks([query])
        hits = await qdrant_search(query_vector=query_vecs[0], k=k)
        if not hits:
            return "（无相关 Wiki 知识）"

        parts = []
        for hit in hits:
            payload = hit.get("payload", {})
            text = payload.get("text", "")
            url = payload.get("url", "")
            source = payload.get("source", "")
            if text:
                src = f"[{source}]({url})" if url else source
                parts.append(f"--- 来源: {src} ---\n{text}")
        return "\n\n".join(parts) if parts else "（无相关 Wiki 知识）"
    except Exception:
        return "（Wiki 知识库查询失败）"


async def author_node(state: CourseState, ch: Chapter, *, client) -> Chapter:
    """Author: 为单个章节生成 Markdown 讲义正文。"""
    if state.sink:
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="author",
            message=f"Author 正在检索 Wiki 为章节「{ch.title}」...",
        ))

    wiki_context = await _build_author_context(ch, k=5)

    if state.sink:
        hit_count = wiki_context.count("--- 来源:")
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="author",
            message=f"Author 找到 {hit_count} 个相关片段，开始撰写讲义...",
        ))

    user_prompt = AUTHOR_USER.format(
        ch_title=ch.title,
        objectives=", ".join(ch.objectives),
        prereqs=", ".join(ch.prereqs) or "无",
        key_concepts=", ".join(ch.key_concepts),
        context=wiki_context,
        k=5,
    )
    resp = client.messages.create(
        model=model_for("sonnet"),
        max_tokens=4096,
        system=AUTHOR_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )

    thinking = extract_thinking(resp)
    if thinking and state.sink:
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="author",
            message=f"Author 推理过程:\n{thinking[:2000]}",
        ))

    ch.content_md = response_text(resp)
    ch.status = "done"
    return ch
