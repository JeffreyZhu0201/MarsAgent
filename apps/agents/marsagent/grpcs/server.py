"""WikiRetriever gRPC 服务实现 + server 工厂。
M1 只实现 Ping；M2 起补 HybridSearch / GetChunks。
"""
from __future__ import annotations

import asyncio
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

    async def HybridSearch(
        self, request, context
    ):
        from marsagent.collector.chunker import _get_model
        from marsagent.rag.qdrant import COLLECTION_NAME, _get_client, qdrant_search

        try:
            client = _get_client()
            names = [c.name for c in client.get_collections().collections]
            if COLLECTION_NAME not in names:
                return wiki_pb2.HybridSearchResp(hits=[])
            if client.count(collection_name=COLLECTION_NAME, exact=False).count == 0:
                return wiki_pb2.HybridSearchResp(hits=[])

            model = _get_model()
            loop = asyncio.get_event_loop()
            query_vec = await loop.run_in_executor(
                None,
                lambda: model.encode([request.query], normalize_embeddings=True)[0].tolist(),
            )
            hits = await qdrant_search(query_vector=query_vec, k=request.k or 10)
        except Exception:
            return wiki_pb2.HybridSearchResp(hits=[])

        pb_hits = []
        for hit in hits:
            payload = hit["payload"]
            pb_hits.append(wiki_pb2.SearchHit(
                doc_id=payload.get("doc_id", ""),
                chunk_id=hit["id"],
                text=payload.get("text", ""),
                score=hit["score"],
                url=payload.get("url", ""),
                source=payload.get("source", ""),
                title="",
            ))
        return wiki_pb2.HybridSearchResp(hits=pb_hits)

    async def GetChunks(
        self, request, context
    ):
        from marsagent.rag.qdrant import _get_client, COLLECTION_NAME

        client = _get_client()
        points = client.retrieve(collection_name=COLLECTION_NAME, ids=request.chunk_ids)
        chunks = []
        for p in points:
            chunks.append(wiki_pb2.Chunk(
                id=p.id,
                doc_id=p.payload.get("doc_id", ""),
                chunk_idx=p.payload.get("chunk_idx", 0),
                text=p.payload.get("text", ""),
                url=p.payload.get("url", ""),
                source=p.payload.get("source", ""),
            ))
        return wiki_pb2.GetChunksResp(chunks=chunks)


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
