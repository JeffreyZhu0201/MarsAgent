"""Quiz 节点 — Haiku 生成习题。"""
from __future__ import annotations

import json

from .state import Chapter, CourseState
from .prompts import QUIZ_SYSTEM, QUIZ_USER


async def quiz_node(state: CourseState, ch: Chapter, *, client) -> Chapter:
    """Quiz: 为章节生成 3 道习题（1 MCQ + 1 填空 + 1 简答）。"""
    user_prompt = QUIZ_USER.format(
        ch_title=ch.title,
        concepts=", ".join(ch.key_concepts),
        summary=ch.content_md[:300] if ch.content_md else "(空)",
    )
    resp = client.messages.create(
        model="claude-haiku-4-5-20251001",
        max_tokens=1024,
        system=QUIZ_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )
    raw = resp.content[0].text
    try:
        start = raw.find("[")
        end = raw.rfind("]") + 1
        ch.quiz = json.loads(raw[start:end])
    except Exception:
        ch.quiz = []
    return ch