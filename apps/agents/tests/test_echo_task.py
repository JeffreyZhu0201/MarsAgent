"""echo task 的单元测试。
不依赖真实 Redis；通过 fake progress sink 验证回调顺序与内容。
"""
import asyncio
import json
from dataclasses import dataclass, field
from unittest.mock import AsyncMock, patch

import pytest

from marsagent.tasks.echo import handle_echo


@dataclass
class FakeSink:
    events: list[dict] = field(default_factory=list)

    async def emit(self, event: dict) -> None:
        self.events.append(event)


@pytest.mark.asyncio
async def test_handle_echo_emits_start_progress_done():
    # 加速 sleep by mocking asyncio.sleep
    mock_sleep = AsyncMock(return_value=None)
    with patch("marsagent.tasks.echo.asyncio.sleep", mock_sleep):
        sink = FakeSink()
        args = json.dumps({"msg": "hi"}).encode()
        await handle_echo(task_id="t-1", args=args, sink=sink)

    types = [e["type"] for e in sink.events]
    assert types[0] == "agent.start"
    assert "agent.progress" in types
    assert types[-1] == "task.done"
    # 所有事件都带 task_id
    assert all(e["task_id"] == "t-1" for e in sink.events)
