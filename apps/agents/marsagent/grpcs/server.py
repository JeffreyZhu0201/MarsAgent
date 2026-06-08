"""WikiRetriever gRPC 服务实现 + server 工厂。
M1 只实现 Ping；M2 起补 HybridSearch / GetChunks。
"""
from __future__ import annotations

import grpc

from marsagent.config import get_settings
from marsagent.gen import wiki_pb2, wiki_pb2_grpc


class WikiRetrieverServicer(wiki_pb2_grpc.WikiRetrieverServicer):
    def __init__(self, server_version: str) -> None:
        self._version = server_version

    async def Ping(
        self,
        request: wiki_pb2.PingReq,
        context: grpc.aio.ServicerContext,
    ) -> wiki_pb2.PingResp:
        return wiki_pb2.PingResp(echo=request.msg, server_version=self._version)


async def build_grpc_server(port: int | None = None) -> tuple[grpc.aio.Server, int]:
    """返回 (server, bound_port)。port=0 时由系统分配（便于测试）。"""
    settings = get_settings()
    server = grpc.aio.server()
    wiki_pb2_grpc.add_WikiRetrieverServicer_to_server(
        WikiRetrieverServicer(server_version=settings.server_version), server,
    )
    listen_port = settings.agents_grpc_port if port is None else port
    bound = server.add_insecure_port(f"[::]:{listen_port}")
    return server, bound
