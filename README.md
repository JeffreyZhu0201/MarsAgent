# MarsAgent

AI 自动建立计算机类课程系统 —— 多智能体（信息收集 + 5 角色建课）+ 后端 Docker 沙箱 + React 三视图前端。

设计文档：[docs/superpowers/specs/2026-06-08-marsagent-design.md](docs/superpowers/specs/2026-06-08-marsagent-design.md)

## 快速启动 (M1)

```bash
# 1. 起基础设施
make infra-up

# 2. 起 gateway / agents / web（三个终端或后台）
make dev
```

打开 http://localhost:5173，在「建课工作台」点击 *Send Echo*，应看到 SSE 进度事件流。

## 目录

- `apps/gateway/` — Go 网关（Gin + Redis Streams + gRPC client）
- `apps/agents/` — Python 智能体 worker（FastAPI + LangGraph + gRPC server）
- `apps/web/` — React 前端（Vite + TS + Tailwind）
- `proto/` — gRPC 契约
- `infra/` — docker-compose 与本地基础设施
- `docs/` — 设计与实施计划
