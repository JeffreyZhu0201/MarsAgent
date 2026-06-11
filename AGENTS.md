# MarsAgent — AI Agent 开发指南

本文档供 Cursor / Claude 等 AI Agent 在本仓库中协作开发时参考。

## 项目概述

MarsAgent 是一个 **AI 自动化建课系统**：用户输入课程主题，5 个 Agent（Planner → Author → CodeEng → Quiz → Validator）协作产出带讲义、代码示例和 Quiz 的完整课程。同时包含 Wiki 知识库采集、在线判题（OJ）和 Docker 代码沙箱。

## 架构速览

```
React 前端 (:5173)
    ↓ HTTP / SSE
Go Gateway (:8080)  — Gin 路由、Redis Streams、gRPC 客户端、OJ 判题
    ↓                    ↓
PostgreSQL          Python Workers (:8001)  — LangGraph 建课、Wiki 采集、gRPC
Redis Streams              ↓
Qdrant 向量库         Docker 沙箱（预热容器池）
MinIO 对象存储
```

## 目录结构

| 路径 | 说明 |
|------|------|
| `apps/gateway/` | Go HTTP 网关，所有 REST/SSE 入口 |
| `apps/gateway/internal/api/` | Gin 路由与 handler |
| `apps/gateway/internal/oj/` | 在线判题：题目、提交、沙箱判题 |
| `apps/gateway/internal/sandbox/` | Go 侧 Docker 沙箱（OJ 判题用） |
| `apps/gateway/internal/store/` | PostgreSQL CRUD（课程、Wiki 草稿） |
| `apps/gateway/internal/stream/` | Redis Streams 生产者/消费者 |
| `apps/agents/marsagent/builder/` | 5-Agent 建课 LangGraph DAG |
| `apps/agents/marsagent/collector/` | Wiki 多源采集适配器 |
| `apps/agents/marsagent/sandbox/` | Python 预热容器池（`/api/sandbox/run` 代理目标） |
| `apps/web/src/` | React 前端（Vite + TanStack Router） |
| `infra/` | Docker Compose、PostgreSQL init.sql |
| `proto/` | gRPC proto 定义 |

## 本地开发

```bash
# 1. 环境变量
cp infra/.env.example infra/.env   # 填入 LLM_API_KEY

# 2. 基础设施
make infra-up

# 3. 三个终端分别启动
make gateway    # :8080
make agents     # :8001
make web        # :5173
```

**注意**：本地开发时 `REDIS_URL` 和 `DATABASE_URL` 需指向 `localhost`，而非 Docker 内部 hostname。

## 测试

```bash
# 全部测试（需 infra-up 且 PostgreSQL 可连）
make test

# 单独运行
cd apps/gateway && go test ./...                          # Go 单元 + 集成
cd apps/gateway && go test ./internal/oj/...              # OJ 纯单元测试
cd apps/gateway && go test ./tests/ -run TestOJ           # OJ 集成（需 PG）
cd apps/agents && PYTHONPATH=. pytest -q                  # Python 单元测试
cd apps/web && npx playwright test                        # E2E
```

集成测试默认连接 `postgres://mars:mars_dev_pw@localhost:5432/marsagent`。

## 代码规范

### 通用原则

- **最小改动**：只改与任务相关的文件，不做无关重构
- **匹配现有风格**：Go 用标准库 + Gin；Python 用 type hints + FastAPI；前端用 Tailwind
- **中文注释**：新增/修改的核心业务逻辑用中文注释（模块 docstring、非显而易见的业务规则）
- **不主动提交**：除非用户明确要求，不要 `git commit` / `git push`

### Go（gateway）

- Handler 通过 `api.Deps` 注入依赖，不在 handler 内 `sql.Open`
- Store 层只做 CRUD，判题逻辑放 `internal/oj/judge.go`
- 测试：`internal/oj/*_test.go` 放纯函数单元测试；`tests/*_test.go` 放需 DB 的集成测试
- 命令前缀 `rtk`（见 `CLAUDE.md`）：`rtk go test ./...`

### Python（agents）

- 入口：`marsagent/main.py`（FastAPI lifespan 管理 Redis、gRPC、容器池）
- 任务 handler 注册在 `StreamConsumer.register()`
- 测试用 `pytest`，异步用 `@pytest.mark.asyncio`
- 格式化：`ruff format`

### 前端（web）

- 路由：`src/routes.tsx`
- API 封装：`src/lib/api.ts`
- SSE Hook：`src/lib/useSse.ts`
- OJ 页面：`ProblemList.tsx`、`ProblemDetail.tsx`、`SubmissionHistory.tsx`

## 关键 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/courses` | 创建建课任务 |
| GET | `/api/stream/:task_id` | SSE 进度流 |
| POST | `/api/wiki/collect` | 触发 Wiki 采集 |
| POST | `/api/sandbox/run` | 代码沙箱（代理到 Python :8001） |
| GET | `/api/oj/problems` | 题目列表 |
| POST | `/api/oj/submissions` | 提交代码（异步判题） |
| GET | `/api/oj/submissions/:id` | 查询判题结果 |

## OJ 模块说明

判题流程：

1. `POST /api/oj/submissions` → 写入 `submissions` 表（status=pending）
2. `JudgeEngine.JudgeSubmissionAsync` 后台 goroutine 判题
3. 逐条测试点调用 `sandbox.Scheduler.Run`（Go Docker 沙箱）
4. `CompareOutput` 比对输出（精确 + 浮点容差）
5. 结果写入 `submission_results`，更新 submission 最终状态

数据表定义见 `infra/postgres/init.sql`（problems、test_cases、submissions、submission_results）。

## 沙箱双路径

| 路径 | 实现 | 用途 |
|------|------|------|
| `POST /api/sandbox/run` | Gateway 代理 → Python `ContainerPool` | 课程代码示例交互执行 |
| OJ 判题 | Go `sandbox.Scheduler` | 每次提交创建临时容器 |

两者均支持 `python` / `node` / `go`，有 64KB 代码上限和 30s 超时。

## 常见任务指引

### 新增 REST 端点

1. 在 `apps/gateway/internal/api/` 或对应 package 写 handler
2. 在 `router.go` 的 `NewRouter` 注册路由
3. 如需 DB，在 `store/` 或 `oj/store.go` 加 CRUD
4. 写 `tests/*_test.go` 集成测试

### 新增 Agent 节点

1. 在 `apps/agents/marsagent/builder/` 新建 node 文件
2. 在 `graph.py` 加入 DAG 边
3. 通过 `progress.py` 的 sink emit `agent.thinking` 事件
4. 写 `tests/test_*.py`

### 修改数据库 Schema

1. 更新 `infra/postgres/init.sql`
2. 重启 infra 或手动 migration
3. 同步更新 Go store 和 Python storage

## 环境变量（关键）

| 变量 | 说明 |
|------|------|
| `LLM_API_KEY` / `LLM_BASE_URL` | Anthropic 兼容 LLM 端点 |
| `DATABASE_URL` | PostgreSQL 连接串 |
| `REDIS_URL` | Redis 连接串 |
| `QDRANT_URL` | 向量库地址 |
| `EMBEDDING_MODE` | `hash`（开发）或 `bge`（生产） |

完整列表见 `infra/.env.example`。

## 参考文档

- 用户文档：[README.md](README.md)
- 设计规格：`docs/superpowers/specs/`
- 实施计划：`docs/superpowers/plans/`（M1–M5）
- RTK 命令优化：[CLAUDE.md](CLAUDE.md)
