"""语义切片 + bge-m3 embedding。"""
from __future__ import annotations

import asyncio
import hashlib
import math
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
    if os.getenv("EMBEDDING_MODE", "hash").lower() != "bge":
        return [_hash_embedding(chunk) for chunk in chunks]

    model = _get_model()
    loop = asyncio.get_event_loop()
    vectors = await loop.run_in_executor(
        None,
        lambda: model.encode(chunks, normalize_embeddings=True).tolist(),
    )
    return vectors


def _hash_embedding(text: str) -> list[float]:
    """Fast deterministic 1024-dim embedding for local smoke/RAG plumbing.

    Set EMBEDDING_MODE=bge to use the real bge-m3 model.
    """
    vec = [0.0] * 1024
    for token in text.lower().split():
        digest = hashlib.sha256(token.encode()).digest()
        idx = int.from_bytes(digest[:2], "big") % len(vec)
        sign = 1.0 if digest[2] % 2 == 0 else -1.0
        vec[idx] += sign
    norm = math.sqrt(sum(v * v for v in vec)) or 1.0
    return [v / norm for v in vec]
