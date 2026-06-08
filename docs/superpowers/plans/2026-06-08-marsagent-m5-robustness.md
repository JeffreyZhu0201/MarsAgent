# MarsAgent M5 — Robustness, Cleanup, and Demo Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the M1-M4 demo reliable enough for daily use: failed jobs become visible, transient failures retry then move to DLQ, SSE closes on terminal failure, sandbox input/output limits are enforced, build artifacts stop polluting the repo, and README documents the full demo path.

**Architecture:** Keep the current Go gateway + Python worker + React SPA architecture. M5 adds defensive behavior at boundaries rather than new product features: Redis Stream retry/DLQ in Python, terminal progress semantics shared with Go/React, stricter sandbox validation in Go, TypeScript no-emit cleanup in React, and smoke-test/README updates.

**Tech Stack:** Python 3.11 + Redis Streams, Go/Gin/Docker SDK, React/Vite/TypeScript, Playwright, Docker Compose.

---

## File Structure

```
MarsAgent/
├── .gitignore                         # modify: ignore TS build info and generated JS artifacts
├── README.md                          # modify: M5 full demo instructions
├── Makefile                           # modify: test targets aligned with current tools
├── apps/agents/marsagent/
│   ├── config.py                      # modify: retry/DLQ settings
│   ├── stream/consumer.py             # modify: retry + DLQ + task.failed
│   └── stream/progress.py             # modify: progress stream TTL
├── apps/gateway/internal/
│   ├── api/sse.go                     # modify: close SSE on task.failed
│   └── sandbox/scheduler.go           # modify: limits + truncation flag + non-root user
└── apps/web/
    ├── tsconfig.json                  # modify: noEmit to prevent src/*.js generation
    └── tsconfig.node.json             # modify: noEmit to prevent vite.config.js/d.ts
```

---

## Task M5-0: Stop build artifacts from polluting the repo

**Files:**
- Modify: `.gitignore`
- Modify: `apps/web/tsconfig.json`
- Modify: `apps/web/tsconfig.node.json`
- Delete generated artifacts if present: `apps/web/src/**/*.js`, `apps/web/*.tsbuildinfo`, `apps/web/vite.config.js`, `apps/web/vite.config.d.ts`

- [ ] **Step 1: Add TypeScript noEmit**

Modify `apps/web/tsconfig.json`, inside `compilerOptions` add:

```json
"noEmit": true,
```

Modify `apps/web/tsconfig.node.json`, inside `compilerOptions` add:

```json
"noEmit": true,
```

- [ ] **Step 2: Extend `.gitignore`**

Add below the Node/Vite section:

```gitignore
# TypeScript build artifacts
*.tsbuildinfo
apps/web/src/**/*.js
apps/web/vite.config.js
apps/web/vite.config.d.ts
```

Also replace the broad Go artifact patterns at the end:

```gitignore
# Go run artifacts
gateway
server
```

with anchored patterns that do not ignore `apps/gateway/**` source files:

```gitignore
# Go run artifacts
/gateway
/server
apps/gateway/server
```

- [ ] **Step 3: Remove generated web artifacts**

Run from repo root:

```bash
rm -f apps/web/tsconfig.tsbuildinfo apps/web/tsconfig.node.tsbuildinfo apps/web/vite.config.js apps/web/vite.config.d.ts
find apps/web/src -name '*.js' -delete
```

- [ ] **Step 4: Verify and commit**

Run:

```bash
cd apps/web
npm run build
cd ../gateway
go test ./...
```

Commit:

```bash
git add .gitignore apps/web/tsconfig.json apps/web/tsconfig.node.json
git add -u apps/web apps/gateway
git commit -m "chore: stop committing generated build artifacts"
```

---

## Task M5-1: Python Redis Stream retry + DLQ

**Files:**
- Modify: `apps/agents/marsagent/config.py`
- Modify: `apps/agents/marsagent/stream/consumer.py`

- [ ] **Step 1: Add settings**

Modify `Settings` in `apps/agents/marsagent/config.py`:

```python
    stream_max_attempts: int = 3
    stream_retry_delay_ms: int = 1000
    stream_dlq_suffix: str = ":dlq"
```

- [ ] **Step 2: Parse attempts from envelope**

Modify `StreamConsumer._dispatch` in `apps/agents/marsagent/stream/consumer.py` so it extracts attempts:

```python
attempts = int(env.get("attempts") or 0)
```

Keep the existing `kind`, `task_id`, and `args` behavior.

- [ ] **Step 3: Add retry/DLQ helper methods**

Add these methods to `StreamConsumer`:

```python
    async def _retry_or_dlq(self, *, msg_id: str, env: dict, reason: str) -> None:
        from marsagent.config import get_settings
        settings = get_settings()
        attempts = int(env.get("attempts") or 0) + 1
        task_id = env.get("task_id", "")
        kind = env.get("kind", "unknown")
        env["attempts"] = attempts
        env["last_error"] = reason

        if attempts >= settings.stream_max_attempts:
            await self.rdb.xadd(f"{self.stream}{settings.stream_dlq_suffix}", {"data": json.dumps(env)})
            sink: ProgressSink = RedisProgressSink(rdb=self.rdb, task_id=task_id)
            await sink.emit(make_event(
                type_="task.failed",
                task_id=task_id,
                agent=kind,
                message=f"任务失败，已进入 DLQ: {reason}",
                extra={"attempts": attempts, "dlq": f"{self.stream}{settings.stream_dlq_suffix}"},
            ))
        else:
            if settings.stream_retry_delay_ms > 0:
                await asyncio.sleep(settings.stream_retry_delay_ms / 1000)
            await self.rdb.xadd(self.stream, {"data": json.dumps(env)})
            sink: ProgressSink = RedisProgressSink(rdb=self.rdb, task_id=task_id)
            await sink.emit(make_event(
                type_="agent.retry",
                task_id=task_id,
                agent=kind,
                message=f"任务失败，准备重试 {attempts}/{settings.stream_max_attempts}: {reason}",
                extra={"attempts": attempts},
            ))

        await self.rdb.xack(self.stream, self.group, msg_id)
```

- [ ] **Step 4: Route handler exceptions to retry/DLQ**

In `_dispatch`, keep a copy of `env` from the parsed envelope. Replace the current `except Exception as e` block with:

```python
        except Exception as e:
            log.exception("handler failed", extra={"kind": kind, "task_id": task_id})
            await sink.emit(make_event(
                type_="agent.error", task_id=task_id, agent=kind,
                message=str(e), extra={"attempts": attempts},
            ))
            await self._retry_or_dlq(msg_id=msg_id, env=env, reason=str(e))
            return
        finally:
            # success path only reaches here; retry/DLQ path returns above after ack
            await self.rdb.xack(self.stream, self.group, msg_id)
```

- [ ] **Step 5: Add a focused unit test**

Create `apps/agents/tests/test_stream_retry.py`:

```python
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
```

- [ ] **Step 6: Verify and commit**

Run:

```bash
cd apps/agents
PYTHONPATH=. pytest tests/test_stream_retry.py -q
python -m py_compile marsagent/config.py marsagent/stream/consumer.py
```

Commit:

```bash
git add apps/agents/marsagent/config.py apps/agents/marsagent/stream/consumer.py apps/agents/tests/test_stream_retry.py
git commit -m "feat(agents): add stream retry and DLQ handling"
```

---

## Task M5-2: Terminal progress semantics and SSE close on failure

**Files:**
- Modify: `apps/agents/marsagent/stream/progress.py`
- Modify: `apps/gateway/internal/stream/subscriber.go`
- Modify: `apps/web/src/lib/useSse.ts`
- Modify: `apps/web/src/components/ProgressFeed.tsx`

- [ ] **Step 1: Add Redis progress TTL**

Modify `RedisProgressSink` in `apps/agents/marsagent/stream/progress.py`:

```python
@dataclass
class RedisProgressSink:
    rdb: aioredis.Redis
    task_id: str
    stream_prefix: str = "progress:"
    ttl_seconds: int = 24 * 60 * 60

    async def emit(self, event: dict[str, Any]) -> None:
        key = f"{self.stream_prefix}{self.task_id}"
        await self.rdb.xadd(key, {"data": json.dumps(event)})
        await self.rdb.expire(key, self.ttl_seconds)
```

- [ ] **Step 2: Close SSE on `task.failed`**

Modify `apps/gateway/internal/stream/subscriber.go`:

```go
if ev.Type == "task.done" || ev.Type == "task.failed" {
	return
}
```

- [ ] **Step 3: Close React EventSource on `task.failed`**

Modify `apps/web/src/lib/useSse.ts`:

```ts
if (ev.type === 'task.done' || ev.type === 'task.failed') {
  es.close()
  setClosed(true)
}
```

- [ ] **Step 4: Add UI badges for retry/failed**

Modify `apps/web/src/components/ProgressFeed.tsx` badge map:

```ts
'agent.retry': 'bg-purple-100 text-purple-700',
'task.failed': 'bg-red-200 text-red-800 font-medium',
```

- [ ] **Step 5: Verify and commit**

Run:

```bash
cd apps/gateway && go test ./...
cd ../agents && python -m py_compile marsagent/stream/progress.py
cd ../web && npm run build
```

Commit:

```bash
git add apps/agents/marsagent/stream/progress.py apps/gateway/internal/stream/subscriber.go apps/web/src/lib/useSse.ts apps/web/src/components/ProgressFeed.tsx
git commit -m "feat: treat task.failed as terminal progress state"
```

---

## Task M5-3: Sandbox request limits and truncation flag

**Files:**
- Modify: `apps/gateway/internal/sandbox/scheduler.go`
- Modify: `apps/gateway/internal/api/sandbox.go`

- [ ] **Step 1: Add limits and validation**

In `apps/gateway/internal/sandbox/scheduler.go`, add:

```go
const (
	MaxCodeBytes   = 64 * 1024
	MaxOutputBytes = 64 * 1024
	MaxTimeoutSec  = 30
)

func ValidateRequest(req RunRequest) error {
	if strings.TrimSpace(req.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if len(req.Code) > MaxCodeBytes {
		return fmt.Errorf("code exceeds %d bytes", MaxCodeBytes)
	}
	if req.Timeout < 0 || req.Timeout > MaxTimeoutSec {
		return fmt.Errorf("timeout must be between 0 and %d seconds", MaxTimeoutSec)
	}
	if req.Lang != "" && imageMap[req.Lang] == "" {
		return fmt.Errorf("unsupported lang %q", req.Lang)
	}
	return nil
}
```

Ensure imports include `strings` already.

- [ ] **Step 2: Use non-root user and set truncation flag**

In `ContainerCreate` config, add:

```go
User: "65534:65534",
```

Replace `truncate` with:

```go
func truncate(s string) (string, bool) {
	if len(s) <= MaxOutputBytes {
		return s, false
	}
	return strings.TrimRight(s[:MaxOutputBytes], "\x00"), true
}
```

In `Run`, compute:

```go
stdoutText, stdoutTruncated := truncate(stdout.String())
stderrText, stderrTruncated := truncate(stderr.String())
```

Return:

```go
Stdout: stdoutText,
Stderr: stderrText,
Truncated: stdoutTruncated || stderrTruncated,
```

- [ ] **Step 3: Use validation in handler**

Modify `apps/gateway/internal/api/sandbox.go` after binding:

```go
if err := sandbox.ValidateRequest(req); err != nil {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	return
}
```

Remove the duplicate direct `req.Code == ""` check.

- [ ] **Step 4: Verify and commit**

Run:

```bash
cd apps/gateway
go test ./...
go build ./...
```

Commit:

```bash
git add apps/gateway/internal/sandbox/scheduler.go apps/gateway/internal/api/sandbox.go
git commit -m "feat(gateway): harden sandbox request limits"
```

---

## Task M5-4: README + Makefile demo/test updates

**Files:**
- Modify: `README.md`
- Modify: `Makefile`

- [ ] **Step 1: Update README**

Replace the M1-only instructions with a current demo section:

```markdown
## 一行启动（完整 demo）

```bash
cp infra/.env.example infra/.env
make infra-up
make agents   # 终端 2，需 conda activate marsagent
make gateway  # 终端 3
make web      # 终端 4
```

Open:
- http://localhost:5173/wiki — Wiki 浏览器
- http://localhost:5173/builder — 建课工作台
- http://localhost:5173/reader — 课程阅读器

Notes:
- 没有 API key 时，外部采集/LLM 建课会失败并进入 retry/DLQ；基础 UI 和 sandbox smoke 仍可跑。
- Python 运行环境必须使用 conda env `marsagent`（Python 3.11）。
```

Add a smoke checks section:

```markdown
## Smoke checks

```bash
cd apps/gateway && go test ./...
cd apps/web && npm run build && npm run test:e2e -- --project=chromium tests/course.spec.ts
cd apps/agents && PYTHONPATH=. pytest tests/test_stream_retry.py -q
```
```

- [ ] **Step 2: Update Makefile test target**

Change `test` target to avoid running Playwright specs under Vitest:

```make
test:
	cd apps/gateway && go test ./... && \
	cd ../agents && PYTHONPATH=. pytest -q && \
	cd ../web && npm run build && npm run test:e2e -- --project=chromium tests/course.spec.ts
```

Change `agents` target to remind conda use:

```make
agents:
	cd apps/agents && PYTHONPATH=. uvicorn marsagent.main:app --reload --port 8001
```

- [ ] **Step 3: Verify and commit**

Run:

```bash
make test
```

If full `make test` cannot complete because API keys are absent or external services are unavailable, run the smoke subset from README and record exact skipped item in the final report.

Commit:

```bash
git add README.md Makefile
git commit -m "docs: update M5 demo and smoke test workflow"
```

---

## Task M5-5: Final review and status cleanup

**Files:**
- No planned code changes unless review finds blockers.

- [ ] **Step 1: Run final checks**

Run:

```bash
git status --short
cd apps/gateway && go test ./...
cd ../web && npm run build && npm run test:e2e -- --project=chromium tests/course.spec.ts
cd ../agents && PYTHONPATH=. pytest tests/test_stream_retry.py -q
```

- [ ] **Step 2: Dispatch final review**

Fresh reviewer scope:
- `apps/agents/marsagent/stream/consumer.py`
- `apps/agents/marsagent/stream/progress.py`
- `apps/gateway/internal/sandbox/scheduler.go`
- `apps/gateway/internal/api/sandbox.go`
- `apps/gateway/internal/stream/subscriber.go`
- `.gitignore`, `README.md`, `Makefile`, `apps/web/tsconfig*.json`

Ask for high-confidence bugs only: retry loop correctness, duplicate ack, DLQ routing, terminal progress closure, sandbox validation/truncation, and build artifact cleanup.

- [ ] **Step 3: Fix and commit if needed**

For each high-confidence issue:
1. Patch only the affected files.
2. Rerun relevant checks.
3. Commit with `fix(m5): ...`.

---

## M5 Acceptance Checklist

- [ ] Failed Python stream tasks retry up to configured attempts and then write to `{stream}:dlq`.
- [ ] DLQ failures emit `task.failed` progress events.
- [ ] Go SSE and React EventSource close on both `task.done` and `task.failed`.
- [ ] Progress streams expire after 24h.
- [ ] Sandbox rejects oversized code, unsupported languages, and invalid timeouts.
- [ ] Sandbox returns `truncated: true` when stdout/stderr exceed the cap.
- [ ] TypeScript build no longer emits `.js` files into `apps/web/src` or `vite.config.js`.
- [ ] README documents the current M4/M5 demo path and smoke checks.
- [ ] `go test ./...`, web build, course e2e smoke, and stream retry test pass.

---

## Self-Review

| spec section | coverage |
|---|---|
| §4.1/§5.3 Streams retry/dead-letter | M5-1 |
| §4.2/§5.4 progress robustness | M5-2 |
| §4.3 sandbox safety | M5-3 |
| §7 M5巡检+鲁棒性 | M5-0 through M5-5 |

**Placeholder scan:** No TBD/TODO placeholders. Commands and expected outcomes are explicit.

**Scope note:** Cron巡检/增量更新 is represented here by durable retry/DLQ + documentation/smoke reliability, not a scheduler daemon. A production cron runner can be added after API keys and external-source quotas are configured.
