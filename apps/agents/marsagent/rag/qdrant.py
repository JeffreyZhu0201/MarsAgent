"""Qdrant 向量库客户端。"""
from __future__ import annotations

import asyncio
import os
from typing import Any

from qdrant_client import QdrantClient
from qdrant_client.models import Distance, PointStruct, VectorParams

COLLECTION_NAME = "wiki_chunks"
EMBEDDING_DIM = 1024
_client = None


def _get_client() -> QdrantClient:
    global _client
    if _client is None:
        _client = QdrantClient(
            url=os.getenv("QDRANT_URL", "http://localhost:6333"),
            timeout=10,
        )
    return _client


def ensure_collection():
    """确保 collection 存在（idempotent）。"""
    client = _get_client()
    names = [c.name for c in client.get_collections().collections]
    if COLLECTION_NAME not in names:
        client.create_collection(
            collection_name=COLLECTION_NAME,
            vectors_config=VectorParams(size=EMBEDDING_DIM, distance=Distance.COSINE),
        )


async def qdrant_upsert(
    chunk_id: str,
    vector: list[float],
    payload: dict[str, Any],
):
    """写入 Qdrant。"""
    loop = asyncio.get_event_loop()
    client = _get_client()
    point = PointStruct(id=chunk_id, vector=vector, payload=payload)
    await loop.run_in_executor(
        None,
        lambda: client.upsert(collection_name=COLLECTION_NAME, points=[point]),
    )


async def qdrant_search(
    query_vector: list[float],
    k: int = 10,
) -> list[dict]:
    """搜索 top-k。"""
    loop = asyncio.get_event_loop()
    client = _get_client()
    results = await loop.run_in_executor(
        None,
        lambda: client.search(
            collection_name=COLLECTION_NAME,
            query_vector=query_vector,
            limit=k,
        ),
    )
    return [
        {"id": r.id, "score": r.score, "payload": r.payload}
        for r in results
    ]
