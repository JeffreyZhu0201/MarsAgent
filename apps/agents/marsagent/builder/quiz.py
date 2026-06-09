"""Quiz 节点 — Haiku 生成习题。"""
from __future__ import annotations

import json

from marsagent.llm import extract_thinking, model_for, response_text
from marsagent.stream.progress import make_event

from .state import Chapter, CourseState
from .prompts import QUIZ_SYSTEM, QUIZ_USER


async def quiz_node(state: CourseState, ch: Chapter, *, client) -> Chapter:
    """Quiz: 为章节生成 3 道习题（1 MCQ + 1 填空 + 1 简答）。"""
    if state.sink:
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="quiz",
            message=f"Quiz 正在为「{ch.title}」设计习题...",
        ))

    user_prompt = QUIZ_USER.format(
        ch_title=ch.title,
        concepts=", ".join(ch.key_concepts),
        summary=ch.content_md[:300] if ch.content_md else "(空)",
    )
    resp = client.messages.create(
        model=model_for("haiku"),
        max_tokens=1024,
        system=QUIZ_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )

    thinking = extract_thinking(resp)
    if thinking and state.sink:
        await state.sink.emit(make_event(
            type_="agent.thinking",
            task_id=state.task_id,
            agent="quiz",
            message=f"Quiz 推理过程:\n{thinking[:2000]}",
        ))

    raw = response_text(resp)
    try:
        start = raw.find("[")
        end = raw.rfind("]") + 1
        ch.quiz = json.loads(raw[start:end])
    except Exception:
        ch.quiz = []
    return ch