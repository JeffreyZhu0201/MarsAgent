import json

import pytest

from marsagent.builder.tasks.build import handle_build_course


class TextBlock:
    def __init__(self, text: str):
        self.text = text


class FakeResponse:
    def __init__(self, text: str):
        self.content = [TextBlock(text)]


class FakeMessages:
    def __init__(self, outline_count: int = 1):
        self.calls = []
        self.outline_count = outline_count

    def create(self, *, model, max_tokens, system, messages):
        self.calls.append({
            "model": model,
            "max_tokens": max_tokens,
            "system": system,
            "messages": messages,
        })
        if "课程规划专家" in system:
            return FakeResponse(json.dumps({
                "outline": [
                    {
                        "ch_id": f"ch_{idx:02d}",
                        "title": "Python 异步基础" if idx == 1 else f"进阶章节 {idx}",
                        "objectives": ["理解事件循环", "会使用 async/await"],
                        "prereqs": ["Python 函数"],
                        "est_min": 30,
                        "bloom_level": "apply",
                        "key_concepts": ["event loop", "coroutine"],
                    }
                    for idx in range(1, self.outline_count + 1)
                ]
            }, ensure_ascii=False))
        if "计算机课程讲师" in system:
            return FakeResponse("## 事件循环\n\n讲义正文 [src: https://example.com/async]")
        if "算法教师" in system:
            return FakeResponse(json.dumps([
                {
                    "lang": "python",
                    "title": "hello async",
                    "code": "import asyncio\nprint('ok')",
                    "expected_output": "ok",
                }
            ], ensure_ascii=False))
        if "习题专家" in system:
            return FakeResponse(json.dumps([
                {
                    "type": "mcq",
                    "question": "asyncio 的核心是什么？",
                    "options": ["event loop", "thread only"],
                    "answer": "event loop",
                    "explanation": "事件循环调度协程。",
                }
            ], ensure_ascii=False))
        if "课程质量审计员" in system:
            return FakeResponse(json.dumps({"pass": True, "issues": [], "suggestions": []}))
        raise AssertionError(f"unexpected system prompt: {system}")


class FakeClient:
    def __init__(self, outline_count: int = 1):
        self.messages = FakeMessages(outline_count=outline_count)


class FakeMinio:
    def __init__(self):
        self.buckets = set()
        self.objects = []

    def bucket_exists(self, bucket: str) -> bool:
        return bucket in self.buckets

    def make_bucket(self, bucket: str) -> None:
        self.buckets.add(bucket)

    def put_object(self, bucket, path, data, length, content_type):
        body = data.read().decode("utf-8")
        self.objects.append({
            "bucket": bucket,
            "path": path,
            "body": body,
            "length": length,
            "content_type": content_type,
        })


class FakeConnection:
    def __init__(self):
        self.executed = []

    def execute(self, query, params):
        self.executed.append({"query": str(query), "params": params})


class FakeBegin:
    def __init__(self, conn: FakeConnection):
        self.conn = conn

    def __enter__(self):
        return self.conn

    def __exit__(self, exc_type, exc, tb):
        return False


class FakeEngine:
    def __init__(self):
        self.conn = FakeConnection()

    def begin(self):
        return FakeBegin(self.conn)


class FakeSink:
    def __init__(self):
        self.events = []

    async def emit(self, event):
        self.events.append(event)


@pytest.mark.asyncio
async def test_handle_build_course_generates_course_artifacts_and_marks_ready(monkeypatch):
    client = FakeClient()
    minio = FakeMinio()
    engine = FakeEngine()
    sink = FakeSink()

    monkeypatch.setattr("marsagent.builder.tasks.build.make_client", lambda: client)
    monkeypatch.setattr("marsagent.collector.storage._get_minio", lambda: minio)
    monkeypatch.setattr("marsagent.collector.storage._get_engine", lambda: engine)

    await handle_build_course(
        task_id="task-1",
        args=json.dumps({
            "course_id": "course-1",
            "topic": "Python 异步编程",
            "audience": "Python 开发者",
            "depth": "intermediate",
        }).encode("utf-8"),
        sink=sink,
    )

    assert [event["type"] for event in sink.events] == [
        "agent.start",
        "agent.progress",
        "agent.progress",
        "task.done",
    ]
    assert minio.buckets == {"marsagent"}
    assert len(minio.objects) == 1
    assert minio.objects[0]["bucket"] == "marsagent"
    assert minio.objects[0]["path"] == "courses/course-1/ch_01.md"
    assert "# Python 异步基础" in minio.objects[0]["body"]
    assert "讲义正文" in minio.objects[0]["body"]

    assert len(engine.conn.executed) == 1
    update = engine.conn.executed[0]
    assert "status = 'ready'" in update["query"]
    assert update["params"]["course_id"] == "course-1"
    assert update["params"]["storage_prefix"] == "courses/course-1/"
    outline = json.loads(update["params"]["outline_json"])
    assert outline[0]["ch_id"] == "ch_01"
    assert outline[0]["status"] == "done"
    assert outline[0]["code_examples"][0]["title"] == "hello async"
    assert outline[0]["quiz"][0]["answer"] == "event loop"

    called_models = [call["model"] for call in client.messages.calls]
    assert called_models == [
        "claude-opus-4-8",
        "claude-sonnet-4-6",
        "claude-sonnet-4-6",
        "claude-haiku-4-5-20251001",
        "claude-sonnet-4-6",
    ]


@pytest.mark.asyncio
async def test_handle_build_course_generates_multiple_chapters(monkeypatch):
    client = FakeClient(outline_count=3)
    minio = FakeMinio()
    engine = FakeEngine()
    sink = FakeSink()

    monkeypatch.setattr("marsagent.builder.tasks.build.make_client", lambda: client)
    monkeypatch.setattr("marsagent.collector.storage._get_minio", lambda: minio)
    monkeypatch.setattr("marsagent.collector.storage._get_engine", lambda: engine)
    monkeypatch.setenv("BUILDER_MAX_CHAPTERS", "3")

    await handle_build_course(
        task_id="task-multi",
        args=json.dumps({
            "course_id": "course-multi",
            "topic": "Python 多章节课程",
            "audience": "Python 开发者",
            "depth": "advanced",
        }).encode("utf-8"),
        sink=sink,
    )

    assert [obj["path"] for obj in minio.objects] == [
        "courses/course-multi/ch_01.md",
        "courses/course-multi/ch_02.md",
        "courses/course-multi/ch_03.md",
    ]
    update = engine.conn.executed[0]
    outline = json.loads(update["params"]["outline_json"])
    assert [ch["ch_id"] for ch in outline] == ["ch_01", "ch_02", "ch_03"]
    assert all(ch["code_examples"] for ch in outline)
    assert all(ch["quiz"] for ch in outline)
    assert sink.events[-1]["type"] == "task.done"
    assert "共 3 章" in sink.events[-1]["message"]
