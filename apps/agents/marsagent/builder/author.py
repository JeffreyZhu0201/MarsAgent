"""Author 节点 — Sonnet 生成章节正文。"""
from __future__ import annotations

from marsagent.llm import model_for

from .state import Chapter, CourseState
from .prompts import AUTHOR_SYSTEM, AUTHOR_USER


async def author_node(state: CourseState, ch: Chapter, *, client) -> Chapter:
    """Author: 为单个章节生成 Markdown 讲义正文。"""
    user_prompt = AUTHOR_USER.format(
        ch_title=ch.title,
        objectives=", ".join(ch.objectives),
        prereqs=", ".join(ch.prereqs) or "无",
        key_concepts=", ".join(ch.key_concepts),
        context="（简化版：M4 填入 Wiki RAG context）",
        k=5,
    )
    resp = client.messages.create(
        model=model_for("sonnet"),
        max_tokens=4096,
        system=AUTHOR_SYSTEM,
        messages=[{"role": "user", "content": user_prompt}],
    )
    ch.content_md = resp.content[0].text
    ch.status = "done"
    return ch
