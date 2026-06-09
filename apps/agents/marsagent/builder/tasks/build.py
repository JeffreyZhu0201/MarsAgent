"""handle_build_course: 运行 LangGraph DAG，产出课程 MD 文件到 MinIO。"""
from __future__ import annotations

import io
import json
from dataclasses import asdict

from sqlalchemy import text

from marsagent.builder.graph import build_course_graph
from marsagent.builder.state import CourseState
from marsagent.llm import make_client
from marsagent.stream.progress import make_event


async def handle_build_course(*, task_id: str, args: bytes, sink) -> None:
    """Main entry point for course.build task."""
    payload = json.loads(args.decode() or "{}")
    course_id = payload.get("course_id", "")
    topic = payload.get("topic", "")
    audience = payload.get("audience", "通用")
    depth = payload.get("depth", "intermediate")

    await sink.emit(make_event(
        type_="agent.start",
        task_id=task_id,
        agent="builder",
        message=f"开始建课: {topic}",
    ))

    # Build initial state
    state = CourseState(
        topic=topic,
        audience=audience,
        depth=depth,
        course_id=course_id,
        task_id=task_id,
        current_agent="planner",
        sink=sink,
    )

    client = make_client()

    try:
        # Run DAG
        graph = build_course_graph(client)
        result = await graph.ainvoke(state)
        final_state = result if isinstance(result, CourseState) else CourseState(**dict(result))

        await sink.emit(make_event(
            type_="agent.progress",
            task_id=task_id,
            agent="builder",
            message=f"DAG 完成，共 {len(final_state.outline)} 章",
        ))

        from marsagent.collector.storage import _get_engine, _get_minio
        mc = _get_minio()
        bucket = "marsagent"
        if not mc.bucket_exists(bucket):
            mc.make_bucket(bucket)

        storage_prefix = f"courses/{course_id}/"
        for ch in (final_state.outline or []):
            md = f"# {ch.title}\n\n## 学习目标\n" + \
                 "\n".join(f"- {o}" for o in ch.objectives) + \
                 "\n\n## 正文\n" + (ch.content_md or "")
            path = f"{storage_prefix}{ch.ch_id}.md"
            md_bytes = md.encode("utf-8")
            mc.put_object(bucket, path, io.BytesIO(md_bytes), len(md_bytes), content_type="text/markdown")
            await sink.emit(make_event(
                type_="agent.progress",
                task_id=task_id,
                agent="builder",
                message=f"已写章节 {ch.ch_id}",
            ))

        outline_json = json.dumps([asdict(ch) for ch in final_state.outline], ensure_ascii=False)
        engine = _get_engine()
        with engine.begin() as conn:
            conn.execute(
                text("""
                    UPDATE courses
                    SET status = 'ready',
                        outline_json = CAST(:outline_json AS jsonb),
                        storage_prefix = :storage_prefix,
                        updated_at = now()
                    WHERE id = :course_id
                """),
                {
                    "course_id": course_id,
                    "outline_json": outline_json,
                    "storage_prefix": storage_prefix,
                },
            )

        await sink.emit(make_event(
            type_="task.done",
            task_id=task_id,
            agent="builder",
            message=f"课程构建完成，共 {len(final_state.outline)} 章",
        ))
    except Exception as e:
        from marsagent.collector.storage import _get_engine
        engine = _get_engine()
        with engine.begin() as conn:
            conn.execute(
                text("""
                    UPDATE courses
                    SET status = 'failed', updated_at = now()
                    WHERE id = :course_id
                """),
                {"course_id": course_id},
            )
        await sink.emit(make_event(
            type_="agent.error",
            task_id=task_id,
            agent="builder",
            message=f"建课失败: {e}",
        ))
        raise
