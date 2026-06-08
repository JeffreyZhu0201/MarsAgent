"""handle_build_course: 运行 LangGraph DAG，产出课程 MD 文件到 MinIO。"""
from __future__ import annotations

import asyncio
import json
import os
import io

import anthropic

from marsagent.builder.graph import build_course_graph
from marsagent.builder.state import CourseState
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
    )

    api_key = os.getenv("ANTHROPIC_API_KEY", "")
    client = anthropic.Anthropic(api_key=api_key)

    # Run DAG
    graph = build_course_graph(client)
    # LangGraph compiled graph uses .arun() for async
    final_state = await graph.arun(state)

    await sink.emit(make_event(
        type_="agent.progress",
        task_id=task_id,
        agent="builder",
        message=f"DAG 完成，共 {len(final_state.outline)} 章",
    ))

    # Write chapters to MinIO
    try:
        from marsagent.collector.storage import _get_minio
        mc = _get_minio()
        bucket = "marsagent"
        for ch in (final_state.outline or []):
            md = f"# {ch.title}\n\n## 学习目标\n" + \
                 "\n".join(f"- {o}" for o in ch.objectives) + \
                 "\n\n## 正文\n" + (ch.content_md or "")
            path = f"courses/{course_id}/{ch.ch_id}.md"
            md_bytes = md.encode("utf-8")
            mc.put_object(bucket, path, io.BytesIO(md_bytes), len(md_bytes), content_type="text/markdown")
            await sink.emit(make_event(
                type_="agent.progress",
                task_id=task_id,
                agent="builder",
                message=f"已写章节 {ch.ch_id}",
            ))
    except Exception as e:
        await sink.emit(make_event(
            type_="agent.error",
            task_id=task_id,
            agent="builder",
            message=f"MinIO 写入失败: {e}",
        ))

    await sink.emit(make_event(
        type_="task.done",
        task_id=task_id,
        agent="builder",
        message=f"课程构建完成，共 {len(final_state.outline)} 章",
    ))
