import json

import pytest

from marsagent.collector.base import RawDoc
from marsagent.collector.tasks import collect


class FakeSink:
    def __init__(self):
        self.events = []

    async def emit(self, event):
        self.events.append(event)


class FakeAdapter:
    async def search(self, query, max_results=10):
        yield RawDoc(
            url="https://example.com/a",
            title="A",
            content="# A\nBody",
            source="fake",
            fetched_at="now",
        )


@pytest.mark.asyncio
async def test_collect_creates_draft_by_default(monkeypatch):
    created = []
    monkeypatch.setitem(collect.ADAPTERS, "fake", lambda: FakeAdapter())

    async def fake_write_draft(**kwargs):
        created.append(kwargs)
        return "draft-1"

    def fail_ensure_collection():
        raise AssertionError("draft mode should not initialize the vector collection")

    monkeypatch.setattr("marsagent.collector.tasks.collect.write_wiki_draft", fake_write_draft)
    monkeypatch.setattr("marsagent.collector.tasks.collect.ensure_collection", fail_ensure_collection)
    monkeypatch.setattr("marsagent.collector.tasks.collect.check_url_seen", lambda _: False)
    monkeypatch.setattr("marsagent.collector.tasks.collect.is_content_duplicate", lambda _: False)
    monkeypatch.setattr("marsagent.collector.tasks.collect.mark_url_seen", lambda _: None)
    monkeypatch.setattr("marsagent.collector.tasks.collect.mark_content_seen", lambda _: None)

    sink = FakeSink()
    await collect.handle_collect(
        task_id="11111111-1111-1111-1111-111111111111",
        args=json.dumps({"topic": "A", "sources": ["fake"], "max_per_source": 1}).encode(),
        sink=sink,
    )

    assert created[0]["title"] == "A"
    assert created[0]["content_md"] == "# A\nBody"
    assert any(e.get("extra", {}).get("stage") == "draft_created" for e in sink.events)
