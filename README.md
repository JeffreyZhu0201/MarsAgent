# MarsAgent

AI 自动建立计算机类课程系统 —— 多智能体（信息收集 + 5 角色建课）+ 后端 Docker 沙箱 + React 三视图前端。

设计文档：[docs/superpowers/specs/2026-06-08-marsagent-design.md](docs/superpowers/specs/2026-06-08-marsagent-design.md)

## 一行启动（完整 demo）

```bash
cp infra/.env.example infra/.env
make infra-up
make agents   # 终端 2，需 conda activate marsagent
make gateway  # 终端 3
make web      # 终端 4
```

Open:

- <http://localhost:5173/wiki> — Wiki 浏览器
- <http://localhost:5173/builder> — 建课工作台
- <http://localhost:5173/reader> — 课程阅读器

Notes:

- 没有 API key 时，外部采集/LLM 建课会失败并进入 retry/DLQ；基础 UI 和 sandbox smoke 仍可跑。
- Python 运行环境必须使用 conda env `marsagent`（Python 3.11）。

## Smoke checks

```bash
cd apps/gateway && go test ./...
cd apps/web && npm run build && npm run test:e2e -- --project=chromium tests/course.spec.ts
cd apps/agents && PYTHONPATH=. pytest tests/test_stream_retry.py -q
```

## 目录

| 路径 | 说明 |
|------|------|
| `apps/gateway/`  | Go 网关（Gin + Redis Streams + gRPC client） |
| `apps/agents/`   | Python 智能体 worker（FastAPI + LangGraph + gRPC server） |
| `apps/web/`      | React 前端（Vite + TS + Tailwind） |
| `proto/`         | gRPC 契约（双端 codegen） |
| `infra/`         | docker-compose 与初始化 SQL |
| `docs/specs/`    | 设计文档 |
| `docs/plans/`    | 实施计划（M1–M5） |

## 当前里程碑

**M5 — 鲁棒性与 demo 可靠性** ✅ retry/DLQ、终端进度状态、sandbox 限制、构建产物清理与 smoke workflow 已完成。

当前完整 demo 与 smoke checks 见上方说明。
