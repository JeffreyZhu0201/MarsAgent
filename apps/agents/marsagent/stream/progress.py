"""进度事件 schema 与发送器。
schema 跨语言对齐 Go 端 stream.ProgressEvent（spec §5.4）。
"""
from __future__ import annotations

import json
import time
from dataclasses import dataclass, field
from typing import Any, Protocol

import redis.asyncio as aioredis


class ProgressSink(Protocol):
    """所有向前端推送进度的渠道都实现这个 Protocol。
    生产环境是 RedisProgressSink；测试用 FakeSink。
    """
    async def emit(self, event: dict[str, Any]) -> None: ...


def make_event(
    *,
    type_: str,
    task_id: str,
    agent: str | None = None,
    pct: int | None = None,
    message: str = "",
    extra: dict[str, Any] | None = None,
) -> dict[str, Any]:
    ev: dict[str, Any] = {
        "type": type_,
        "task_id": task_id,
        "ts": int(time.time()),
    }
    if agent is not None:
        ev["agent"] = agent
    if pct is not None:
        ev["pct"] = pct
    if message:
        ev["message"] = message
    if extra:
        ev["extra"] = extra
    return ev


@dataclass
class RedisProgressSink:
    rdb: aioredis.Redis
    task_id: str
    stream_prefix: str = "progress:"

    async def emit(self, event: dict[str, Any]) -> None:
        key = f"{self.stream_prefix}{self.task_id}"
        await self.rdb.xadd(key, {"data": json.dumps(event)})
