"""Anthropic-compatible LLM client and model routing."""
from __future__ import annotations

import os
from typing import Literal

import anthropic

from marsagent.config import get_settings

ModelTier = Literal["haiku", "sonnet", "opus"]


def make_client() -> anthropic.Anthropic:
    """Create an Anthropic-compatible client from env/settings."""
    settings = get_settings()
    api_key = settings.llm_api_key or os.getenv("ANTHROPIC_API_KEY", "")
    base_url = settings.llm_base_url or os.getenv("ANTHROPIC_BASE_URL", "")
    kwargs: dict[str, str] = {"api_key": api_key}
    if base_url:
        kwargs["base_url"] = base_url
    return anthropic.Anthropic(**kwargs)


def model_for(tier: ModelTier) -> str:
    settings = get_settings()
    return {
        "haiku": settings.model_haiku,
        "sonnet": settings.model_sonnet,
        "opus": settings.model_opus,
    }[tier]


def response_text(resp) -> str:
    """Extract visible text from Anthropic-compatible responses.

    Some providers return reasoning/thinking blocks before the final text block. The
    Anthropic SDK can deserialize those provider-specific blocks as content entries
    whose `.text` is None, so callers must scan for actual text instead of assuming
    `content[0].text` is the answer.
    """
    parts: list[str] = []
    for block in getattr(resp, "content", []) or []:
        text = getattr(block, "text", None)
        if isinstance(text, str) and text:
            parts.append(text)
    return "\n".join(parts).strip()


def extract_thinking(resp) -> str:
    """Extract thinking/reasoning content from Anthropic responses.

    Thinking blocks have type='thinking' and a .thinking attribute (not .text).
    Concatenates all thinking blocks into a single string.
    """
    parts: list[str] = []
    for block in getattr(resp, "content", []) or []:
        # ThinkingBlock has type='thinking' and .thinking attribute
        block_type = getattr(block, "type", None)
        if block_type == "thinking":
            thinking = getattr(block, "thinking", None)
            if isinstance(thinking, str) and thinking:
                parts.append(thinking)
    return "\n".join(parts).strip()
