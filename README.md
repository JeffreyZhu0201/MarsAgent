# MarsAgent

AI 自动建立计算机类课程系统 —— 多智能体（信息收集 + 5 角色建课）+ 后端 Docker 沙箱 + React 三视图前端。

设计文档：[docs/superpowers/specs/2026-06-08-marsagent-design.md](docs/superpowers/specs/2026-06-08-marsagent-design.md)

## 一行启动 (M1)

```bash
cp infra/.env.example infra/.env
docker compose -f infra/docker-compose.yml --env-file infra/.env up --build
# 打开 http://localhost:5173/builder ，点 Send Echo
```

## 开发模式

```bash
# 终端 1：起基础设施
make infra-up

# 终端 2：Python worker
source apps/agents/.venv/bin/activate
make agents

# 终端 3：Go gateway
make gateway

# 终端 4：React 前端
make web
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

**M1 — 骨架贯通** ✅ 浏览器点按钮 → SSE 收到 worker 进度。

下一步：[`M2 Wiki 收集 MVP`](docs/superpowers/plans/2026-06-08-marsagent-m2-wiki-collector.md)。
