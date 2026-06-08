"""Haiku 摘要 + 质量打分 + 语言判定。"""
from __future__ import annotations

import json
import os
import re
from dataclasses import dataclass

import anthropic


SYSTEM_PROMPT = (
    "You are a research assistant. Given a web document, produce a concise "
    "summary (3-5 sentences), a quality score (0-10), and the primary language. "
    "Return JSON with fields: summary, quality_score, language."
)


@dataclass
class SummaryResult:
    summary: str
    quality_score: float
    language: str


async def summarize(text: str, url: str) -> SummaryResult:
    client = anthropic.Anthropic(api_key=os.getenv("ANTHROPIC_API_KEY", ""))
    try:
        resp = client.messages.create(
            model="claude-haiku-4-5-20251001",
            max_tokens=512,
            system=SYSTEM_PROMPT,
            messages=[
                {"role": "user", "content": f"Summarize this document from {url}:\n\n{text[:8000]}"}
            ],
        )
        raw = resp.content[0].text
        try:
            data = json.loads(raw)
        except Exception:
            score_m = re.search(r"quality_score[\"s: ]+(\d+)", raw)
            data = {
                "summary": raw[:300],
                "quality_score": float(score_m.group(1) or 5),
                "language": "en",
            }
        return SummaryResult(
            summary=data.get("summary", "")[:1000],
            quality_score=float(data.get("quality_score", 5)),
            language=data.get("language", "en"),
        )
    except Exception:
        return SummaryResult(summary=text[:500], quality_score=3.0, language="en")
