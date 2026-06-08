"""验证 WikiRetriever.Ping 返回 echo + server_version。
启一个临时 gRPC server，用同进程 channel 连接。
"""
from __future__ import annotations

import asyncio
from contextlib import asynccontextmanager

import grpc
import pytest

from marsagent.gen import wiki_pb2, wiki_pb2_grpc
from marsagent.grpcs.server import build_grpc_server


@asynccontextmanager
async def running_server(port: int = 0):
    server, bound_port = await build_grpc_server(port=port)
    await server.start()
    try:
        yield bound_port
    finally:
        await server.stop(grace=0.5)


@pytest.mark.asyncio
async def test_ping_round_trip():
    async with running_server() as port:
        async with grpc.aio.insecure_channel(f"localhost:{port}") as ch:
            stub = wiki_pb2_grpc.WikiRetrieverStub(ch)
            resp = await stub.Ping(wiki_pb2.PingReq(msg="hello"))
            assert resp.echo == "hello"
            assert resp.server_version
