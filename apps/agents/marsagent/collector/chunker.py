"""语义切片 + bge-m3 embedding。"""
from __future__ import annotations

import asyncio
import os
import re
from dataclasses import dataclass

from sentence_transformers import SentenceTransformer

MODEL_NAME = "BAAI/bge-m3"
CHUNK_SIZE = 512
_model = None


def _get_model():
    global _model
    if _model is None:
        _model = SentenceTransformer(MODEL_NAME)
    return _model


@dataclass
class Chunk:
    doc_id: str
    chunk_idx: int
    text: str
    embedding: list[float]


def semantic_chunk(text: str) -> list[str]:
    chunks, current, current_len = [], [], 0
    for para in re.split(r'\n\n+', text):
        tokens = len(para.split())
        if current_len + tokens > CHUNK_SIZE and current:
            chunks.append('\n'.join(current))
            current, current_len = [para], tokens
        else:
            current.append(para)
            current_len += tokens
    if current:
        chunks.append('\n'.join(current))
    return [c.strip() for c in chunks if c.strip()]


async def embed_chunks(chunks: list[str]) -> list[list[float]]:
    model = _get_model()
    loop = asyncio.get_event_loop()
    vectors = await loop.run_in_executor(
        None,
        lambda: model.encode(chunks, normalize_embeddings=True).tolist(),
    )
    return vectors
