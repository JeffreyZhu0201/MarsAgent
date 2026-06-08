"""MinIO 文件存储 + Postgres 元数据写入。"""
from __future__ import annotations

import hashlib as _hl
import io
import os
import uuid
from datetime import datetime, timezone

from minio import Minio
from sqlalchemy import create_engine, text
from sqlalchemy.orm import Session, sessionmaker

from marsagent.config import get_settings


_minio_client = None
_engine = None


def _get_minio() -> Minio:
    global _minio_client
    if _minio_client is None:
        _minio_client = Minio(
            endpoint=os.getenv("MINIO_ENDPOINT", "localhost:9000"),
            access_key=os.getenv("MINIO_ROOT_USER", "minio"),
            secret_key=os.getenv("MINIO_ROOT_PASSWORD", "minio_dev_pw"),
            secure=False,
        )
    return _minio_client


def _get_engine():
    global _engine
    if _engine is None:
        url = os.getenv(
            "DATABASE_URL",
            "postgresql://mars:mars_dev_pw@localhost:5432/marsagent",
        )
        _engine = create_engine(url)
    return _engine


def _slugify(title: str) -> str:
    import re
    slug = re.sub(r'[^a-zA-Z0-9]', '-', title.lower()).strip('-')[:80]
    if slug:
        suffix = _hl.md5(title.encode()).hexdigest()[:4]
        return f"{slug}-{suffix}"
    return _hl.sha256(title.encode()).hexdigest()[:12]


async def write_wiki_doc(
    *,
    title: str,
    content: str,
    url: str,
    url_hash: bytes,
    content_hash: bytes,
    source: str,
    category: str,
    quality_score: float,
    language: str,
) -> tuple[str, str]:
    """写 MD 文件到 MinIO，返回 (doc_id, storage_path)。同时写 wiki_docs 表。"""
    import asyncio
    doc_id = str(uuid.uuid4())
    slug = _slugify(title)
    storage_path = f"wiki/{category}/{slug}.md"

    mc = _get_minio()
    bucket = "marsagent"
    if not mc.bucket_exists(bucket):
        mc.make_bucket(bucket)
    data_bytes = content.encode("utf-8")
    loop = asyncio.get_event_loop()
    await loop.run_in_executor(
        None,
        lambda: mc.put_object(
            bucket, storage_path, io.BytesIO(data_bytes), len(data_bytes),
            content_type="text/markdown",
        ),
    )

    engine = _get_engine()
    with Session(engine) as sess:
        sess.execute(
            text("""
                INSERT INTO wiki_docs
                  (id, slug, category, title, url, url_hash, content_hash,
                   source, quality_score, language, storage_path, fetched_at, updated_at)
                VALUES
                  (:id, :slug, :category, :title, :url, :url_hash, :content_hash,
                   :source, :quality_score, :language, :storage_path, :fetched_at, :updated_at)
                ON CONFLICT (slug) DO UPDATE SET
                  updated_at = EXCLUDED.updated_at,
                  quality_score = EXCLUDED.quality_score
            """),
            {
                "id": doc_id, "slug": slug, "category": category,
                "title": title, "url": url,
                "url_hash": url_hash, "content_hash": content_hash,
                "source": source, "quality_score": quality_score,
                "language": language, "storage_path": storage_path,
                "fetched_at": datetime.now(timezone.utc),
                "updated_at": datetime.now(timezone.utc),
            },
        )
        sess.commit()

    return doc_id, storage_path
