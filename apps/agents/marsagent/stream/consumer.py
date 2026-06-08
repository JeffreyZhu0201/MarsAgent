"""通用 Redis Streams consumer。
用法：
    consumer = StreamConsumer(rdb, stream="wiki:collect:tasks",
                              group="agents-workers", consumer="worker-1")
    consumer.register("echo", handle_echo)
    await consumer.run()
"""
from __future__ import annotations

import asyncio
import json
import logging
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field

import redis.asyncio as aioredis

from marsagent.stream.progress import ProgressSink, RedisProgressSink, make_event

log = logging.getLogger(__name__)

TaskHandler = Callable[..., Awaitable[None]]
# 约定 handler 签名: handle(task_id: str, args: bytes, sink: ProgressSink) -> None


@dataclass
class StreamConsumer:
    rdb: aioredis.Redis
    stream: str
    group: str
    consumer: str
    handlers: dict[str, TaskHandler] = field(default_factory=dict)
    block_ms: int = 5000

    def register(self, kind: str, handler: TaskHandler) -> None:
        self.handlers[kind] = handler

    async def ensure_group(self) -> None:
        try:
            await self.rdb.xgroup_create(
                name=self.stream, groupname=self.group, id="0", mkstream=True,
            )
        except aioredis.ResponseError as e:
            if "BUSYGROUP" not in str(e):
                raise

    async def run(self) -> None:
        await self.ensure_group()
        log.info("consumer started", extra={"stream": self.stream, "group": self.group})
        while True:
            try:
                # First drain messages already pending for this consumer. This recovers work
                # delivered before a worker restart but not acked before process exit.
                pending = await self.rdb.xreadgroup(
                    groupname=self.group, consumername=self.consumer,
                    streams={self.stream: "0"}, count=8, block=1,
                )
                if self._has_messages(pending):
                    await self._dispatch_batch(pending)
                    continue

                resp = await self.rdb.xreadgroup(
                    groupname=self.group, consumername=self.consumer,
                    streams={self.stream: ">"}, count=8, block=self.block_ms,
                )
            except asyncio.CancelledError:
                raise
            except Exception:
                log.exception("xreadgroup failed; sleeping 1s")
                await asyncio.sleep(1)
                continue

            if not resp:
                continue
            await self._dispatch_batch(resp)

    def _has_messages(self, resp) -> bool:
        return any(messages for _stream_name, messages in (resp or []))

    async def _dispatch_batch(self, resp) -> None:
        for _stream_name, messages in resp:
            for msg_id, fields in messages:
                await self._dispatch(msg_id, fields)

    async def _dispatch(self, msg_id: str, fields: dict[bytes, bytes]) -> None:
        raw = fields.get(b"data") or b"{}"
        try:
            env = json.loads(raw)
            kind = env["kind"]
            task_id = env["task_id"]
            attempts = int(env.get("attempts") or 0)
            raw_args = env.get("args") or {}
            args = json.dumps(raw_args).encode() if isinstance(raw_args, dict) else b"{}"
        except Exception:
            log.exception("malformed envelope; acking and dropping", extra={"msg_id": msg_id})
            await self.rdb.xack(self.stream, self.group, msg_id)
            return

        handler = self.handlers.get(kind)
        if handler is None:
            log.warning("no handler for kind", extra={"kind": kind})
            await self.rdb.xack(self.stream, self.group, msg_id)
            return

        sink: ProgressSink = RedisProgressSink(rdb=self.rdb, task_id=task_id)
        try:
            await handler(task_id=task_id, args=args, sink=sink)
        except Exception as e:
            log.exception("handler failed", extra={"kind": kind, "task_id": task_id})
            await sink.emit(make_event(
                type_="agent.error", task_id=task_id, agent=kind,
                message=str(e), extra={"attempts": attempts},
            ))
            await self._retry_or_dlq(msg_id=msg_id, env=env, reason=str(e))
            return

        await self.rdb.xack(self.stream, self.group, msg_id)

    async def _retry_or_dlq(self, *, msg_id: str, env: dict, reason: str) -> None:
        from marsagent.config import get_settings

        settings = get_settings()
        attempts = int(env.get("attempts") or 0) + 1
        task_id = env.get("task_id", "")
        kind = env.get("kind", "unknown")
        env["attempts"] = attempts
        env["last_error"] = reason

        if attempts >= settings.stream_max_attempts:
            dlq = f"{self.stream}{settings.stream_dlq_suffix}"
            await self.rdb.xadd(dlq, {"data": json.dumps(env)})
            sink: ProgressSink = RedisProgressSink(rdb=self.rdb, task_id=task_id)
            await sink.emit(make_event(
                type_="task.failed",
                task_id=task_id,
                agent=kind,
                message=f"任务失败，已进入 DLQ: {reason}",
                extra={"attempts": attempts, "dlq": dlq},
            ))
        else:
            if settings.stream_retry_delay_ms > 0:
                await asyncio.sleep(settings.stream_retry_delay_ms / 1000)
            await self.rdb.xadd(self.stream, {"data": json.dumps(env)})
            sink: ProgressSink = RedisProgressSink(rdb=self.rdb, task_id=task_id)
            await sink.emit(make_event(
                type_="agent.retry",
                task_id=task_id,
                agent=kind,
                message=f"任务失败，准备重试 {attempts}/{settings.stream_max_attempts}: {reason}",
                extra={"attempts": attempts},
            ))

        await self.rdb.xack(self.stream, self.group, msg_id)
