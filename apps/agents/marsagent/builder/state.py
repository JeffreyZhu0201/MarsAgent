"""LangGraph State — 课程构建的共享状态。"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

from marsagent.stream.progress import ProgressSink

@dataclass
class Chapter:
    ch_id: str
    title: str
    objectives: list[str] = field(default_factory=list)
    prereqs: list[str] = field(default_factory=list)
    est_min: int = 30
    bloom_level: str = "understand"
    key_concepts: list[str] = field(default_factory=list)
    content_md: str = ""
    code_examples: list[dict] = field(default_factory=list)
    quiz: list[dict] = field(default_factory=list)
    status: Literal["pending", "writing", "done", "failed"] = "pending"
    retry_count: int = 0

@dataclass
class CourseState:
    """LangGraph 每一步的全局状态。"""
    topic: str
    audience: str = "通用"
    depth: str = "intermediate"
    outline: list[Chapter] = field(default_factory=list)
    course_id: str = ""
    task_id: str = ""
    pct: int = 0
    current_agent: str = ""
    error: str = ""
    sink: "ProgressSink | None" = field(default=None, repr=False)
    def to_dict(self) -> dict: return {"topic": self.topic, "status": self.outline is not None}