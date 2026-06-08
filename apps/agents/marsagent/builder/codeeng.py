"""CodeEng 节点 — Sonnet 生成代码示例。"""
from __future__ import annotations

import json

from .state import Chapter, CourseState
from .prompts import CODEENG_SYSTEM, CODEENG_USER


async def codeeng_node(state: CourseState, ch: Chapter, *, client) -> Chapter:
    """CodeEng: 为章节生成 2-3 个精选代码示例。"""
    user_prompt = CODEENG_USER.format(
        ch_title=ch.title,
        concepts=", ".join(ch.key_concepts),
        summary=ch.content_md[:300] if ch.content_md else "(空)",
    )
    resp = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=2048,
        system=CODEENG_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )
    raw = resp.content[0].text
    try:
        start = raw.find("[")
        end = raw.rfind("]") + 1
        examples = json.loads(raw[start:end])
        ch.code_examples = examples
    except Exception:
        ch.code_examples = []
    return ch