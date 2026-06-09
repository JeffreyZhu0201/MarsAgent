# MarsAgent

**AI 自动化建课系统** — 多智能体协作，从主题输入到完整课程产出。

输入一个课程主题 → 5 个 Agent 并行/串行工作 → 输出带讲义、代码示例、Quiz 的可阅读课程。

---

## 目录

- [特性](#特性)
- [架构](#架构)
- [技术栈](#技术栈)
- [快速启动](#快速启动)
- [项目结构](#项目结构)
- [配置](#配置)
- [API 与端口](#api-与端口)
- [核心流程详解](#核心流程详解)
- [开发命令](#开发命令)

---

## 特性

### 建课工作流（Course Builder）

- **5 Agent 协作管道**：Planner → Author → CodeEng → Quiz → Validator
- **实时推理可视化**：每个 Agent 的 LLM 思考过程（thinking block）实时展示在前端紫色推理面板
- **真实 Wiki RAG**：课程大纲和章节正文均基于 [Qdrant](https://qdrant.tech) 向量数据库检索到的相关 Wiki 知识生成
- **流式 SSE 进度**：前端实时显示每个 Agent 的启动、推理、完成的各阶段事件
- **自动重试与 DLQ**：LLM 调用失败自动重试 3 次，永久失败进入 Dead Letter Queue

### Wiki 知识库

- **多源采集**：[Tavily](https://tavily.com)（网页搜索）、[ArXiv](https://arxiv.org)（学术论文）、Wikipedia、[GitHub](https://github.com) README
- **JS 渲染页面抓取**：[Playwright](https://playwright.dev) 抓取需浏览器渲染的内容
- **语义切片**：512 token 段落级语义分块
- **向量存储**：[BGE-M3](https://huggingface.co/BAAI/bge-m3) embedding → [Qdrant](https://qdrant.tech) ANN 向量检索
- **审稿工作流**：采集的 Wiki 草稿需人工审核发布（草稿 → 已发布）
- **Wiki 搜索 API**：支持 gRPC 混合搜索（向量+关键词）

### Sandbox（代码执行）

- **Docker 隔离执行**：Python / JavaScript / Go 代码在独立 Docker 容器运行
- **超时与内存限制**：防止恶意/无限循环代码
- **标准输出/错误捕获**：实时返回执行结果

---

## 架构

```txt
┌─────────────────────────────────────────────────────────────┐
│                        Frontend (React)                      │
│   /wiki  ·  /builder  ·  /reader                           │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP / SSE

┌──────────────────▼──────────────────────────────────────────┐
│                     Go Gateway (:8080)                       │
│   Gin Router  ·  Redis Streams  ·  gRPC Client  ·  REST API  │
└──────┬────────────────────┬────────────────────┬────────────┘
       │                    │                    │
       ▼                    ▼                    ▼
┌─────────────┐    ┌─────────────────┐   ┌──────────────┐
│  PostgreSQL │    │  Python Workers │   │   Qdrant     │
│  (drafts,   │    │  (FastAPI +     │   │  (ANN 向量库) │
│   courses)  │    │   LangGraph)    │   └──────────────┘
└─────────────┘    │   :8001 gRPC   │            ▲
                   │                │            │ embed + search
                   │  ┌───────────┐  │   ┌─────────┴────────┐
                   │  │ 5-Agent  │  │   │  Collector Agents │
                   │  │  Course   │  │   │  (Tavily/ArXiv/   │
                   │  │ Builder  │  │   │   GitHub/Wiki)    │
                   │  └───────────┘  │   └──────────────────┘
                   │                 │
                   │  ┌───────────┐  │
                   │  │  SSE      │  │
                   │  │  Stream   │  │
                   │  └───────────┘  │
                   └─────────────────┘

┌─────────────────────────────────────────────────────────────┐
│              Infrastructure (Docker Compose)                   │
│   PostgreSQL :5432  ·  Redis :6379  ·  Qdrant :6333        │
│   MinIO :9000  ·  (可选: Browserless for Playwright)         │
└─────────────────────────────────────────────────────────────┘
```

### 数据流

1. **用户** 在前端输入课程主题 → POST `/api/courses`
2. **Gateway** 将任务写入 Redis Stream → 返回 `task_id`
3. **Worker** 消费任务 → 5-Agent DAG 协作建课
4. **Planner** 先查 Qdrant（真实 RAG）→ 生成大纲 JSON
5. **Author/CodeEng/Quiz/Validator** 串行处理每章节
6. **各节点** 实时 emit `agent.thinking` 事件 → Redis Stream → SSE → 前端
7. **完成后** 课程写入 PostgreSQL，章节 MD 存入 MinIO

---

## 技术栈

| 层 | 技术 |
|----|------|
| 前端 | [React](https://react.dev) · [Vite](https://vitejs.dev) · [TypeScript](https://www.typescriptlang.org) · [Tailwind CSS](https://tailwindcss.com) · [TanStack Router](https://tanstack.com/router) |
| 网关 | [Go](https://go.dev) · [Gin](https://gin-gonic.com) · [Redis Streams](https://redis.io/docs/data-streams/) · [gRPC](https://grpc.io) |
| 智能体 | [Python 3.11](https://docs.python.org/3.11/) · [FastAPI](https://fastapi.tiangolo.com) · [LangGraph](https://langchain-ai.github.io/langgraph/) · [Anthropic SDK](https://anthropic.com/docs) |
| 向量库 | [Qdrant](https://qdrant.tech) v1.11 |
| 关系库 | [PostgreSQL](https://www.postgresql.org) 16 |
| 缓存/队列 | [Redis](https://redis.io) 7 |
| 对象存储 | [MinIO](https://min.io)（兼容 S3） |
| 沙箱 | [Docker](https://www.docker.com) + sysbox/rootless |

---

## 快速启动

### 前提

- [Docker](https://docs.docker.com/get-docker/) · [Docker Compose](https://docs.docker.com/compose/install/)
- [Python 3.11](https://docs.python.org/3.11/)（建议 conda/venv）
- [Node.js](https://nodejs.org) 18+
- [Go](https://go.dev/doc/install) 1.21+

### 步骤

```bash
# 1. 克隆后，进入项目根目录
cd MarsAgent

# 2. 复制环境变量
cp infra/.env.example infra/.env
# 编辑 infra/.env 填入你的 API Key（LLM_API_KEY）

# 3. 启动基础设施（PostgreSQL / Redis / Qdrant / MinIO）
make infra-up

# 4. 终端 2 — 启动 Python Agent Worker
cd apps/agents
make agents

# 5. 终端 3 — 启动 Go Gateway
cd apps/gateway
make gateway

# 6. 终端 4 — 启动 React 前端
cd apps/web
make web
```

访问（浏览器打开）：

- <http://localhost:5173/wiki> — Wiki 知识库浏览器
- <http://localhost:5173/builder> — 建课工作台（5 Agent 实时推理可视化）
- <http://localhost:5173/reader> — 课程阅读器

---

## 项目结构

```txt
MarsAgent/
├── apps/
│   ├── gateway/              # Go HTTP 网关 + gRPC client
│   │   ├── cmd/server/      # 入口点
│   │   └── internal/
│   │       ├── api/          # Gin 路由、处理器（SSE、REST、gRPC 代理）
│   │       ├── config/       # 环境变量配置
│   │       ├── grpcc/        # gRPC 客户端（WikiRetriever、CourseBuilder）
│   │       ├── sandbox/      # Docker Sandbox 调度器
│   │       ├── store/        # PostgreSQL CRUD（courses、drafts、wiki_docs）
│   │       └── stream/       # Redis Streams 生产者/消费者
│   │
│   ├── agents/               # Python 多智能体 Worker
│   │   ├── marsagent/
│   │   │   ├── builder/      # 5-Agent 课程构建 DAG
│   │   │   │   ├── planner.py       # Planner（查 RAG → 生成大纲）
│   │   │   │   ├── author.py        # Author（写章节正文）
│   │   │   │   ├── codeeng.py       # CodeEng（生成代码示例）
│   │   │   │   ├── quiz.py          # Quiz（生成练习题）
│   │   │   │   ├── validator.py     # Validator（质量审计）
│   │   │   │   ├── graph.py         # LangGraph DAG 定义
│   │   │   │   ├── state.py         # CourseState / Chapter dataclass
│   │   │   │   ├── prompts.py       # 各 Agent 的 System/User prompt
│   │   │   │   └── tasks/build.py   # 入口：消费 Redis 任务 → 运行 DAG
│   │   │   ├── collector/     # Wiki 采集（多源适配器）
│   │   │   │   ├── tavily_adapter.py   # Tavily 搜索
│   │   │   │   ├── arxiv_adapter.py    # ArXiv 论文
│   │   │   │   ├── github_adapter.py   # GitHub README
│   │   │   │   ├── doc_adapter.py      # Wikipedia
│   │   │   │   ├── playwright_adapter.py  # JS 渲染页面
│   │   │   │   ├── chunker.py     # 512-token 语义分块 + BGE embedding
│   │   │   │   └── storage.py      # 写 drafts / wiki_docs 表
│   │   │   ├── rag/
│   │   │   │   └── qdrant.py       # Qdrant ANN 向量检索
│   │   │   ├── grpcs/
│   │   │   │   └── server.py       # gRPC server（HybridSearch RPC）
│   │   │   ├── stream/
│   │   │   │   └── progress.py     # Redis Progress Sink（emit SSE 事件）
│   │   │   ├── llm.py              # Anthropic SDK 封装 + thinking 提取
│   │   │   └── main.py             # FastAPI 入口
│   │   └── tests/
│   │
│   └── web/                 # React 前端
│       └── src/
│           ├── views/
│           │   ├── CourseBuilder.tsx  # 建课工作台 + 实时推理面板
│           │   ├── WikiBrowser.tsx    # Wiki 知识库浏览器
│           │   └── CourseReader.tsx   # 课程阅读器
│           ├── components/
│           │   ├── ThinkingPanel.tsx  # LLM 推理过程可视化面板
│           │   ├── ProgressFeed.tsx   # 实时 SSE 事件流组件
│           │   ├── AgentOrbit.tsx     # 5-Agent 轨道动画
│           │   └── MarkdownView.tsx   # Markdown 渲染
│           └── lib/
│               ├── api.ts      # 所有 REST API 调用
│               └── useSse.ts  # SSE 流式进度 Hook
│
├── infra/
│   ├── docker-compose.dev.yml  # 基础设施编排
│   ├── .env.example           # 环境变量模板
│   └── sql/                   # PostgreSQL schema
│
├── proto/
│   └── wiki.proto             # WikiRetriever gRPC 服务定义
│
└── docs/
    └── superpowers/
        ├── specs/              # 设计文档
        └── plans/              # M1–M5 实施计划
```

---

## 配置

### 必需的环境变量（[`infra/.env`](infra/.env.example) / [`apps/agents/.env`](apps/agents/.env)）

```bash
# LLM（Anthropic 兼容端点，火山引擎 Ark 为例）
LLM_BASE_URL=https://ark.cn-beijing.volces.com/api/coding
LLM_API_KEY=ark-xxxxx
MODEL_HAIKU=minimax-m2.7
MODEL_SONNET=deepseek-v4-pro
MODEL_OPUS=deepseek-v4-pro

# Embedding（RAG 向量化）
EMBEDDING_MODE=hash    # 开发用快速 hash；生产设为 bge

# 向量数据库
QDRANT_URL=http://localhost:6333

# Redis
REDIS_URL=redis://localhost:6379/0

# 数据库
DATABASE_URL=postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable

# 建课参数
BUILDER_MAX_CHAPTERS=3    # 最多生成章节数
```

> **注意**：`apps/agents/.env` 由 `cp infra/.env apps/agents/.env` 自动生成。`infra/.env` 中的 `REDIS_URL=redis://redis:6379/0` 供 Docker 内服务通信用，本地开发时需改为 `redis://localhost:6379/0`。

### Embedding 模式

| 模式 | 说明 |
|------|------|
| `hash` | 快速确定性 hash（开发/测试用，无需 GPU） |
| `bge` | [BAAI/bge-m3](https://huggingface.co/BAAI/bge-m3) 模型（生产用，需 GPU） |

---

## API 与端口

| 端口 | 服务 | 主要端点 |
|------|------|---------|
| `:5173` | [Vite](https://vitejs.dev) 前端 | `/` |
| `:8080` | [Go Gateway](apps/gateway/) | 见下 |
| `:8001` | [FastAPI Agents](apps/agents/) | `/docs` |
| `:6333` | [Qdrant](https://qdrant.tech) Dashboard | `http://localhost:6333/dashboard` |

### Gateway REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/wiki/collect` | 触发 Wiki 采集任务 |
| GET | `/api/wiki/tree` | 获取 Wiki 文档树 |
| POST | `/api/wiki/search` | 搜索 Wiki（RAG 向量检索） |
| POST | `/api/courses` | 创建建课任务 |
| GET | `/api/courses` | 列出所有课程 |
| GET | `/api/courses/:id` | 获取课程详情 |
| GET | `/api/courses/:id/chapter/:ch_id` | 获取章节 Markdown |
| POST | `/api/sandbox/run` | 在 Docker 沙箱中执行代码 |
| GET | `/api/stream/:task_id` | SSE 流，实时推送 `agent.*` 事件 |

### SSE 事件类型

| 事件 | 说明 |
|------|------|
| `agent.start` | 任务开始 |
| `agent.thinking` | LLM 推理中（包含完整 thinking block 内容） |
| `agent.progress` | 进度更新 |
| `agent.error` | 错误 |
| `agent.retry` | 重试中 |
| `agent.done` | 当前 Agent 完成 |
| `task.done` | 整任务完成 |
| `task.failed` | 任务永久失败，进入 DLQ |

---

## 核心流程详解

### 建课流程（Course Builder）

```
用户点击"开始建课"
  │
  ▼
POST /api/courses → task_id
  │
  ▼
┌─────────────────────────────────────────────────────────┐
│  Planner Agent（查 Wiki RAG → 生成 JSON 大纲）          │
│   1. 查询 Qdrant top-20 相关 chunk                     │
│   2. emit agent.thinking（SSE 推送推理过程）            │
│   3. Opus 生成大纲 JSON                                 │
│   4. emit agent.thinking（推送 LLM thinking 内容）     │
└─────────────────────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────────────────────┐
│  Author Agent（每章节：查 RAG → 写讲义）                │
│   1. 查询 Qdrant top-5 相关 chunk                       │
│   2. emit agent.thinking                               │
│   3. Sonnet 生成章节 Markdown 正文                     │
└─────────────────────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────────────────────┐
│  CodeEng → Quiz → Validator（各自 emit thinking）       │
└─────────────────────────────────────────────────────────┘
  │
  ▼
课程写入 PostgreSQL + MinIO，task.done 推送至前端
```

相关源码：[`planner.py`](apps/agents/marsagent/builder/planner.py) · [`author.py`](apps/agents/marsagent/builder/author.py) · [`graph.py`](apps/agents/marsagent/builder/graph.py)

### Wiki 采集流程

```
POST /api/wiki/collect { topic: "Python 异步" }
  │
  ▼
┌──────────────────────────────────────────┐
│  多源并发采集（Tavily / ArXiv / GitHub）  │
│   1. TavilyAdapter.search() → RawDoc[]  │
│   2. ArxivAdapter.search()  → RawDoc[]  │
│   3. GitHubAdapter.search()  → RawDoc[]  │
└──────────────────────────────────────────┘
  │
  ▼
┌──────────────────────────────────────────┐
│  PlaywrightAdapter.fetch()（可选 JS 渲染）│
└──────────────────────────────────────────┘
  │
  ▼
语义分块 (chunker.py)
  │
  ▼
BGE-M3 embedding / Hash embedding
  │
  ▼
Upsert to Qdrant（wiki_chunks collection）
  │
  ▼
WikiDraft 写入 PostgreSQL（待审核）
```

相关源码：[`collector/`](apps/agents/marsagent/collector/) · [`chunker.py`](apps/agents/marsagent/collector/chunker.py) · [`qdrant.py`](apps/agents/marsagent/rag/qdrant.py)

### Wiki RAG 检索

```
planner_node() / author_node()
  │
  ▼
embed_chunks([query])        # BGE-M3 or Hash embedding
  │
  ▼
qdrant_search(query_vec, k=20)   # ANN cosine similarity
  │
  ▼
构建 wiki_context 字符串（含来源 URL）
  │
  ▼
注入 PLANNER_USER / AUTHOR_USER prompt
  │
  ▼
LLM 生成内容（大纲/正文）
```

---

## 开发命令

```bash
# 基础设施
make infra-up      # 启动 Docker（PG / Redis / Qdrant / MinIO）
make infra-down    # 停止
make infra-logs    # 查看日志

# 各服务（需分别在不同终端）
make gateway       # Go Gateway (:8080)
make agents       # Python Worker (:8001)
make web          # Vite 前端 (:5173)

# 测试
make test         # 跑全部测试（Go + Python + Playwright）

# 代码格式化
make fmt          # go fmt + ruff format + npm format

# Proto 代码生成
make proto        # 重新生成 Go + Python gRPC 代码
```

### 端到端测试

```bash
cd apps/web
npx playwright test                  # 所有 Playwright 测试
npx playwright test course.spec.ts   # 只跑建课测试
npx playwright test wiki-drafts.spec.ts  # 只跑 Wiki 草稿测试
```

### Python 调试

```bash
cd apps/agents
PYTHONPATH=. .venv/bin/python -c "
from marsagent.builder.tasks.build import handle_build_course
# ... 导入并直接调用调试
"
```

---

## 模型配置说明

系统使用 **[Anthropic 兼容 SDK](https://anthropic.com/docs)**，可对接任何 OpenAI-compatible 端点（[火山引擎 Ark](https://www.volcengine.com/docs/82379/1399009)、[OpenRouter](https://openrouter.ai)、[DeepSeek](https://platform.deepseek.com) 等）。

### 验证模型可用性

```python
import anthropic
client = anthropic.Anthropic(api_key="your-key", base_url="your-base-url")
resp = client.messages.create(model="模型名", max_tokens=10,
    messages=[{"role": "user", "content": "hi"}])
print(resp.content[0].text)
```

### 角色说明

| Tier | 用途 | 推荐模型 |
|------|------|---------|
| `haiku` | Quiz（快速生成习题） | `minimax-m2.7` |
| `sonnet` | Author / CodeEng / Validator（中等推理） | `deepseek-v4-pro` |
| `opus` | Planner（复杂规划，需要 4096+ tokens 输出） | `deepseek-v4-pro` |

---

## 相关链接

| 资源 | 链接 |
|------|------|
| 火山引擎 Ark（LLM 平台） | <https://www.volcengine.com/docs/82379/1399009> |
| Qdrant 文档 | <https://qdrant.tech/documentation/> |
| LangGraph 文档 | <https://langchain-ai.github.io/langgraph/> |
| Anthropic SDK | <https://anthropic.com/docs> |
| BGE-M3 Embedding | <https://huggingface.co/BAAI/bge-m3> |
| Playwright | <https://playwright.dev> |
