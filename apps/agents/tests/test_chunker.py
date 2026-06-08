from marsagent.collector.chunker import embed_chunks


async def test_hash_embedding_is_fast_1024_dim(monkeypatch):
    monkeypatch.setenv("EMBEDDING_MODE", "hash")

    vectors = await embed_chunks(["hello world"])

    assert len(vectors) == 1
    assert len(vectors[0]) == 1024
    assert any(v != 0 for v in vectors[0])
