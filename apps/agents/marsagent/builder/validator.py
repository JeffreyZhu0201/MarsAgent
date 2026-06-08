"""Validator 节点 — Sonnet 审计章节质量。"""
from __future__ import annotations

import json

from marsagent.llm import model_for, response_text

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
        model=model_for("sonnet"),
        max_tokens=1024,
        system=VALIDATOR_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )
    raw = response_text(resp)
    try:
        start = raw.find("{")
        end = raw.rfind("}") + 1
        verdict = json.loads(raw[start:end])
        if not verdict.get("pass", True):
            # MVP 不让 validator 阻塞整门课生成；问题记录到 state.error，
            # 后续 M5+ 可在 UI 上展示并触发人工/自动修订。
            state.error = "; ".join(verdict.get("issues", []))
    except Exception:
        pass  # validator 解析失败不阻塞，视为通过
    return ch
