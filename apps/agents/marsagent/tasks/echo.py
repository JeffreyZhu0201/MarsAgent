"""echo task：演示用，验证 stream 双向连通。
依次发 5 条 progress (每秒 1 条) + 最终 task.done。
"""
from __future__ import annotations

import asyncio
import json

from marsagent.stream.progress import ProgressSink, make_event


async def handle_echo(*, task_id: str, args: bytes, sink: ProgressSink) -> None:
    payload = json.loads(args.decode() or "{}")
    msg = payload.get("msg", "")

    await sink.emit(make_event(
        type_="agent.start", task_id=task_id, agent="echo",
        message=f"received: {msg!r}",
    ))
    for i in range(1, 5):
        await asyncio.sleep(1)
        await sink.emit(make_event(
            type_="agent.progress", task_id=task_id, agent="echo",
            pct=i * 20, message=f"step {i}/5",
        ))
    await sink.emit(make_event(
        type_="task.done", task_id=task_id, agent="echo",
        message="echo complete",
    ))
