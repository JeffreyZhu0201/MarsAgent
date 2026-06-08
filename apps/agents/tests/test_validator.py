import json

import pytest

from marsagent.builder.state import Chapter, CourseState
from marsagent.builder.validator import validator_node


class Block:
    def __init__(self, text):
        self.text = text


class Resp:
    def __init__(self, text):
        self.content = [Block(text)]


class Messages:
    def create(self, **kwargs):
        return Resp(json.dumps({"pass": False, "issues": ["缺少引用"], "suggestions": []}, ensure_ascii=False))


class Client:
    messages = Messages()


@pytest.mark.asyncio
async def test_validator_records_issue_without_blocking_course_completion():
    state = CourseState(topic="Python")
    chapter = Chapter(ch_id="ch_01", title="Intro", objectives=["learn"], content_md="body", status="done")

    result = await validator_node(state, chapter, client=Client())

    assert result.status == "done"
    assert state.error == "缺少引用"
