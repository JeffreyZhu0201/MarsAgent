"""基于 URL hash 的精确去重 + simhash 近似去重。"""
from __future__ import annotations

import hashlib

import simhash

# In-memory dedup state (M5 迁移到 Redis HyperLogLog)
_known_url_hashes: set[bytes] = set()
_known_simhashes: dict[int, bool] = {}


def compute_hashes(text: str, url: str) -> tuple[bytes, int]:
    url_hash = hashlib.sha256(url.encode()).digest()
    content_simhash_int = simhash.Simhash(text).value
    return url_hash, content_simhash_int


def check_url_seen(url_hash: bytes) -> bool:
    return url_hash in _known_url_hashes


def mark_url_seen(url_hash: bytes):
    _known_url_hashes.add(url_hash)


def is_content_duplicate(content_simhash_int: int) -> bool:
    for known, _ in _known_simhashes.items():
        dist = (content_simhash_int ^ known).bit_count()
        if dist <= 3:
            return True
    return False


def mark_content_seen(content_simhash_int: int):
    _known_simhashes[content_simhash_int] = True
