"""Planner 节点 — 使用 Opus 生成课程大纲。"""
from __future__ import annotations

import json
import anthropic

from marsagent.config import get_settings
from marsagent.llm import model_for, response_text

from .state import Chapter, CourseState
from .prompts import PLANNER_SYSTEM, PLANNER_USER


async def llm_json(client: anthropic.Anthropic, system: str, user: str) -> dict:
    resp = client.messages.create(
        model=model_for("opus"),
        max_tokens=4096,
        system=system,
        messages=[{"role": "user", "content": user}],
    )
    raw = response_text(resp)
    start = raw.find("{")
    end = raw.rfind("}") + 1
    return json.loads(raw[start:end])


async def planner_node(state: CourseState, *, client: anthropic.Anthropic, rag_top_k: int = 20) -> CourseState:
    """Planner: 查 Wiki → 生成大纲。"""
    # M3 用 text-only search，Wiki RAG context 简化
    user_prompt = PLANNER_USER.format(
        topic=state.topic,
        audience=state.audience,
        depth=state.depth,
        wiki_context="（Wiki 搜索在 M4 补全；M3 用 LLM 自身知识）",
    )
    result = await llm_json(client, PLANNER_SYSTEM, user_prompt)
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