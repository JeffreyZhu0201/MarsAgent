"""worker 进程入口。
- FastAPI 提供 /healthz（liveness）。
- lifespan 启动期：拉 Redis 连接、起 StreamConsumer、起 gRPC server。
- lifespan 关闭期：优雅停。
"""
from __future__ import annotations

import asyncio
import logging
from contextlib import asynccontextmanager

import redis.asyncio as aioredis
from fastapi import FastAPI

from marsagent.config import get_settings
from marsagent.grpcs.server import build_grpc_server
from marsagent.stream.consumer import StreamConsumer
from marsagent.collector.tasks.collect import handle_collect
from marsagent.tasks.echo import handle_echo
from marsagent.builder.tasks.build import handle_build_course

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
log = logging.getLogger("marsagent")


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    rdb = aioredis.from_url(settings.redis_url, decode_responses=False)

    # 1) gRPC server
    grpc_server, bound_port = await build_grpc_server()
    await grpc_server.start()
    log.info("grpc listening on :%d", bound_port)

    # 2) Stream consumers
    wiki_consumer = StreamConsumer(
        rdb=rdb,
        stream="wiki:collect:tasks",
        group=settings.stream_group,
        consumer=settings.stream_consumer,
    )
    wiki_consumer.register("echo", handle_echo)
    wiki_consumer.register("wiki.collect", handle_collect)

    course_consumer = StreamConsumer(
        rdb=rdb,
        stream="course:build:tasks",
        group=settings.stream_group,
        consumer=f"{settings.stream_consumer}-course",
    )
    course_consumer.register("course.build", handle_build_course)

    consumer_tasks = [
        asyncio.create_task(wiki_consumer.run(), name="wiki-collect-consumer"),
        asyncio.create_task(course_consumer.run(), name="course-build-consumer"),
    ]

    app.state.rdb = rdb
    app.state.grpc = grpc_server
    app.state.consumer_tasks = consumer_tasks
    try:
        yield
    finally:
        log.info("shutting down...")
        for task in consumer_tasks:
            task.cancel()
        for task in consumer_tasks:
            try:
                await task
            except asyncio.CancelledError:
                pass
        await grpc_server.stop(grace=2)
        await rdb.aclose()
        log.info("shutdown complete")


app = FastAPI(lifespan=lifespan, title="MarsAgent Worker")


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}
