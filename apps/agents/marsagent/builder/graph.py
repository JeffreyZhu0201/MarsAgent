"""LangGraph DAG — 5-Agent 课程构建工作流。"""
from __future__ import annotations

from langgraph.graph import END, StateGraph

from .author import author_node
from .codeeng import codeeng_node
from .planner import planner_node
from .quiz import quiz_node
from .state import Chapter, CourseState
from .validator import validator_node


def build_course_graph(client) -> StateGraph:
    g = StateGraph(CourseState)

    # Planner: generates course outline
    async def planner_wrapper(state: CourseState) -> CourseState:
        return await planner_node(state, client=client)

    g.add_node("planner", planner_wrapper)

    # Per-chapter fan-out wrappers
    def chapter_fanout(node_fn):
        async def wrapper(state: CourseState) -> CourseState:
            chapters = state.outline or []
            results = [await node_fn(state, ch, client=client) for ch in chapters]
            state.outline = list(results)
            return state
        return wrapper

    g.add_node("author",   chapter_fanout(author_node))
    g.add_node("codeeng",  chapter_fanout(codeeng_node))
    g.add_node("quiz",     chapter_fanout(quiz_node))
    g.add_node("validator", chapter_fanout(validator_node))

    g.set_entry_point("planner")
    g.add_edge("planner", "author")
    g.add_edge("author", "codeeng")
    g.add_edge("codeeng", "quiz")
    g.add_edge("quiz", "validator")

    # Retry: if any chapter failed and retry_count < 2, go back to author
    def retry_or_done(state: CourseState) -> str:
        for ch in (state.outline or []):
            if ch.status == "failed" and ch.retry_count < 2:
                ch.retry_count += 1
                return "author"
        return "__end__"

    g.add_conditional_edges("validator", retry_or_done, {"author": "author", "__end__": END})

    return g.compile()
