"""Validator 节点 — Sonnet 审计章节质量。"""
from __future__ import annotations

import json

from .state import Chapter, CourseState
from .prompts import VALIDATOR_SYSTEM, VALIDATOR_USER


async def validator_node(state: CourseState, ch: Chapter, *, client) -> Chapter:
    """Validator: 审计章节内容质量。"""
    user_prompt = VALIDATOR_USER.format(
        ch_title=ch.title,
        objectives=", ".join(ch.objectives),
        content_md=ch.content_md or "(空)",
    )
    resp = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=1024,
        system=VALIDATOR_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )
    raw = resp.content[0].text
    try:
        start = raw.find("{")
        end = raw.rfind("}") + 1
        verdict = json.loads(raw[start:end])
        if not verdict.get("pass", True):
            ch.status = "failed"
            state.error = "; ".join(verdict.get("issues", []))
    except Exception:
        pass  # validator 解析失败不阻塞，视为通过
    return ch
