import json

import pytest

from marsagent.stream.consumer import StreamConsumer


class FakeRedis:
    def __init__(self):
        self.added = []
        self.acked = []

    async def xadd(self, stream, values):
        self.added.append((stream, values))

    async def xack(self, stream, group, msg_id):
        self.acked.append((stream, group, msg_id))

    async def expire(self, key, seconds):
        self.expired = (key, seconds)


@pytest.mark.asyncio
async def test_retry_or_dlq_writes_dlq_after_max_attempts(monkeypatch):
    class Settings:
        stream_max_attempts = 1
        stream_retry_delay_ms = 0
        stream_dlq_suffix = ":dlq"

    monkeypatch.setattr("marsagent.config.get_settings", lambda: Settings())
    rdb = FakeRedis()
    c = StreamConsumer(rdb=rdb, stream="course:build:tasks", group="g", consumer="c")
    await c._retry_or_dlq(
        msg_id="1-0",
        env={"kind": "course.build", "task_id": "t1", "args": {}},
        reason="boom",
    )
    assert rdb.added[0][0] == "course:build:tasks:dlq"
    payload = json.loads(rdb.added[0][1]["data"])
    assert payload["attempts"] == 1
    assert rdb.acked == [("course:build:tasks", "g", "1-0")]


def test_has_messages_distinguishes_empty_redis_stream_response():
    c = StreamConsumer(rdb=FakeRedis(), stream="course:build:tasks", group="g", consumer="c")

    assert not c._has_messages([])
    assert not c._has_messages([[b"course:build:tasks", []]])
    assert c._has_messages([[b"course:build:tasks", [(b"1-0", {b"data": b"{}"})]]])
