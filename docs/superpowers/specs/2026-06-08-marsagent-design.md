# MarsAgent — AI 自动建立计算机类课程系统 · 设计文档

- **日期**：2026-06-08
- **作者**：brainstorming session（人 + Claude）
- **状态**：Draft v1 · 待评审
- **参考**：[SamurAIGPT/llm-wiki-agent](https://github.com/SamurAIGPT/llm-wiki-agent)

---

## 1. 概述

MarsAgent 是一个多智能体系统，目标是**全自动**为计算机类课程产出**交互式在线课程**（含可运行代码、quiz、lab）。系统由两组智能体协作：

1. **信息收集 Agent** — 从全网（搜索引擎、技术博客、arXiv、官方文档、GitHub 等）持续采集最新资料，沉淀为可检索的 **LLM Wiki 知识库**（Markdown + 向量 RAG）。
2. **建课 Agent** — 基于 Wiki，按 5 角色 DAG（规划师 / 章节作者 / 代码工程师 / 习题专家 / 质量校验）协同生成完整课程。

后端采用 **Go 网关 + Python Worker 混合架构**，前端是 **React 三视图 SPA**（Wiki 浏览器 / 建课工作台 / 课程阅读器）。代码示例通过**后端 Docker 容器沙箱**实际运行。

MVP 阶段**无认证、单租户**，但数据库结构预留 `user_id` 字段，便于后续接入外部系统底座。

---

## 2. 设计决策摘要

| # | 决策 | 选择 |
|---|------|------|
| 1 | 课程形态 | 交互式在线课程型（含可运行代码 + quiz + lab） |
| 2 | 信息源 & 工作模式 | 全网搜索 + 技术博客 + 定时巡检 + 按需触发 |
| 3 | 后端语言 | **混合**：Go 网关 + Python Agent Worker |
| 4 | Wiki 存储/检索 | Markdown 文件 + Qdrant 向量 + Postgres 元数据（RAG） |
| 5 | 建课 Agent 结构 | 5 角色：Planner / Author / CodeEng / Quiz / Validator |
| 6 | 前端形态 | 三视图 SPA：Wiki 浏览器 / 建课工作台 / 课程阅读器 |
| 7 | Go ↔ Python 通信 | **Redis Streams**（异步任务）+ **gRPC**（同步检索） |
| 8 | 代码沙箱 | 后端 Docker 容器（一次性、cgroup 限额、可叠加 gVisor） |
| 9 | LLM 选型 | 分层：Haiku 4.5（轻）+ Sonnet 4.6（主）+ Opus 4.8（规划/裁决）+ Prompt cache |
| 10 | 认证/多租户 | 无认证·单租户 MVP（预留 `user_id`） |

---

## 3. 整体架构

```
┌───────────────────────────────────────────────────────────────────────────┐
│  Browser — React SPA (Vite + Tailwind + shadcn/ui + react-markdown)        │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐    │
│  │ Wiki 浏览器     │  │ 建课工作台      │  │ 课程阅读器              │    │
│  │ 目录/检索/详情  │  │ 5-Agent 进度    │  │ Monaco + Quiz + Run     │    │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────────────┘    │
└───────────┼────────────────────┼────────────────────┼─────────────────────┘
            │ HTTP / SSE         │ HTTP / SSE         │ HTTP
            └──────────┬─────────┴────────────────────┘
                       ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  Go Gateway (Gin)                                                          │
│  REST + SSE + Docker Sandbox Scheduler + Redis Streams Producer            │
└────────┬───────────────────────────────┬──────────────────────────────────┘
         │ gRPC (同步检索)               │ Redis Streams (异步任务/进度)
         ▼                               ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  Python Agent Workers (FastAPI + LangGraph + anthropic-sdk)                │
│  ┌──────────────────────────┐  ┌───────────────────────────────────────┐  │
│  │ 信息收集 Agent           │  │ 建课 Agent (5 角色 LangGraph DAG)     │  │
│  │ Tavily/arXiv/GitHub/MDN/ │  │ Planner → (Author ‖ CodeEng ‖ Quiz)   │  │
│  │ Playwright + Haiku 摘要  │  │ → Validator → 章节产物                │  │
│  │ + dedup + embed → Qdrant │  │ (per-chapter fanout, Prompt cache)    │  │
│  └────────────┬─────────────┘  └─────────────────┬─────────────────────┘  │
└───────────────┼────────────────────────────────────┼──────────────────────┘
                │ writes                             │ reads (RAG)
                ▼                                    ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  Storage: Postgres · Qdrant · MinIO/FS · Redis (Streams + cache)           │
└───────────────────────────────────────────────────────────────────────────┘
```

各层职责：

- **前端**：单 SPA，三个路由视图共享 shadcn/ui 组件与 SSE hook。
- **Go Gateway**：所有外部入口、Docker 沙箱调度（通过 host docker.sock 启一次性容器）、SSE 进度桥、Redis Streams Producer。
- **Python Workers**：两个 Agent 系统（信息收集 / 建课），都基于 LangGraph 编排；通过 Redis Streams 消费异步任务，通过 gRPC 对外暴露同步 RAG 检索。
- **存储**：Postgres（结构化元数据，含预留 `user_id`）+ Qdrant（chunk 向量）+ MinIO 或本地文件系统（Markdown / 课程产物）+ Redis（Streams、cache、分布式锁）。

---

## 4. 数据流

### 4.1 信息收集流程

触发方式：Cron（每日巡检热门主题）或 `POST /api/wiki/collect {topic}`。

```
Go 写 Stream: wiki:collect:tasks {task_id, topic, depth, sources}
   │
   ▼
Python Worker 消费 → 并发跑 Source Collector:
   ├─ TavilySearch / SerpAPI       (搜索 → URL 列表 + snippet)
   ├─ arXiv API                    (论文元数据 + 摘要)
   ├─ GitHub API                   (README / Awesome lists)
   ├─ Playwright Scraper           (技术博客，需 JS 渲染)
   └─ Wikipedia / MDN / 官方文档
   │
   ▼
归一化为 RawDoc{url, title, content, source, fetched_at, raw_html}
   │
   ▼
Haiku 摘要 + 质量打分 + 主题分类 + 语言判定
   │
   ▼
去重: simhash 邻近 + Postgres unique(url_hash, content_hash)
   │
   ▼
切片 (semantic chunking, ~512 token) → bge-m3 embedding → Qdrant
   │
   ▼
写 MD 文件: /wiki/{category}/{slug}.md  + Postgres 元数据
   │
   ▼
进度事件写 Stream: wiki:progress:{task_id} → Go SSE → 前端
```

### 4.2 建课流程（5-Agent LangGraph DAG）

触发：`POST /api/courses {topic, audience, depth}`。

```
LangGraph State: { topic, audience, outline, chapters[], retries{} }

① Planner (Opus)
   · RAG: Wiki 顶层主题 hybrid 检索 (BM25 + 向量 + rerank)
   · 产出 outline: [{ch_id, title, objectives, prereqs, est_min,
                     bloom_level, key_concepts[]}]
        │
        ▼  fan-out: for ch in outline.chapters (并发 N=4)
   ┌────────────────┬────────────────┬────────────────┐
   ▼                ▼                ▼
   ② Author        ③ CodeEng        ④ Quiz
   (Sonnet)        (Sonnet)         (Haiku)
   · RAG chunk      · 生成代码 +     · 按 Bloom 难度
     + cache         预期输出         生成 MCQ/填空/lab
   · 讲义 MD         · runtime/deps   · 答案 + 解析
   └────────────────┴────────────────┴────────────────┘
        │ Join per-chapter → ChapterDoc
        ▼
⑤ Validator (Sonnet)
   · 概念覆盖度 vs outline.key_concepts
   · 引用必须在 Wiki（防幻觉）
   · 代码 lint（不执行；运行交 Docker 沙箱按需）
   · 章节衔接、术语一致性
   · 不合格 → 回 ② 重试（per-chapter 最多 2 次）
        │
        ▼
产物: /courses/{course_id}/
      ├── meta.json           (outline + metadata)
      ├── 01-intro.md ... NN-final.md
      └── assets/             (图、代码片段、quiz JSON)
        │
        ▼
进度事件: course:progress:{task_id} → Go SSE → 前端工作台
```

### 4.3 代码运行（学习时触发）

```
前端 [Run] → POST /api/sandbox/run {language, code, stdin, timeout}
   │
   ▼
Go SandboxScheduler:
   · 限流: per-IP token bucket + 全局并发上限 (default 8)
   · 选镜像: python:3.11 / node:20 / go:1.22 / ...
   · docker run --rm --network=none --memory=256m --cpus=0.5
                --pids-limit=64 --read-only --tmpfs /tmp:size=64m
                --ulimit nproc=64 --user 65534 --stop-timeout 10
                <image> sh -c "<bootstrap> ; timeout 15 <run>"
   · 收集 stdout/stderr/exit_code (上限 1MB, 超出截断)
   · 可选 gVisor (runsc) 增强隔离
   │
   ▼
返回 {stdout, stderr, exit_code, duration_ms, truncated}
```

---

## 5. 组件与接口

### 5.1 REST API（Go Gateway）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/wiki/tree` | Wiki 目录树 |
| GET  | `/api/wiki/doc/{slug}` | 单文档 MD + 元数据（来源、更新时间） |
| POST | `/api/wiki/search` | Hybrid 检索（body: `{q, k}`） |
| POST | `/api/wiki/collect` | 触发收集任务，返回 `{task_id}` |
| GET  | `/api/stream/{task_id}` | SSE 进度事件流（统一通道） |
| POST | `/api/courses` | 触发建课，返回 `{course_id, task_id}` |
| GET  | `/api/courses` | 课程列表 |
| GET  | `/api/courses/{id}` | 课程 outline + 章节列表 |
| GET  | `/api/courses/{id}/chapter/{ch_id}` | 章节 MD + quiz JSON |
| POST | `/api/sandbox/run` | 提交代码执行，返回执行结果 |

### 5.2 gRPC（Go → Python，同步检索）

```proto
service WikiRetriever {
  rpc HybridSearch (SearchReq)  returns (SearchResp);
  rpc GetChunks    (GetChunksReq) returns (ChunksResp);
}

message SearchReq  { string q = 1; int32 k = 2; repeated string filters = 3; }
message SearchHit  { string doc_id = 1; string chunk_id = 2; string text = 3;
                     float score = 4; string url = 5; string source = 6; }
message SearchResp { repeated SearchHit hits = 1; }
```

### 5.3 Redis Streams 拓扑

| Stream | 方向 | 用途 |
|--------|------|------|
| `wiki:collect:tasks` | Go → Py worker | 收集任务 |
| `wiki:progress:{task_id}` | Py → Go | 收集进度 |
| `course:build:tasks` | Go → Py worker | 建课任务 |
| `course:progress:{task_id}` | Py → Go | 建课进度（含每个 Agent 节点状态） |

所有 task stream 用 consumer group + ack + DLQ（M5 补）。

### 5.4 SSE 事件 schema（统一）

```json
{ "type": "agent.start", "agent": "planner",  "task_id": "...", "ts": 1700000000 }
{ "type": "agent.progress", "agent": "author", "chapter_id": "ch_02", "pct": 40 }
{ "type": "agent.error", "agent": "validator", "chapter_id": "ch_03", "error": "..." }
{ "type": "agent.done", "agent": "planner",  "result_url": "/api/courses/.../outline" }
{ "type": "task.done", "task_id": "...", "course_id": "..." }
```

### 5.5 数据模型（节选）

```sql
-- Postgres
create table wiki_docs (
  id uuid primary key,
  user_id uuid null,                    -- 预留
  slug text unique not null,
  category text not null,
  title text not null,
  url text not null,
  url_hash bytea not null,
  content_hash bytea not null,
  source text not null,                 -- tavily/arxiv/github/...
  quality_score real,
  language text,
  fetched_at timestamptz not null,
  updated_at timestamptz not null,
  storage_path text not null,           -- MinIO 路径
  unique (url_hash),
  unique (content_hash)
);

create table courses (
  id uuid primary key,
  user_id uuid null,                    -- 预留
  topic text not null,
  audience text,
  depth text,
  status text not null,                 -- pending|building|ready|failed
  outline_json jsonb,
  storage_prefix text not null,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table tasks (
  id uuid primary key,
  user_id uuid null,                    -- 预留
  kind text not null,                   -- collect|build_course
  ref_id uuid,                          -- course_id or null
  status text not null,
  error text,
  created_at timestamptz default now()
);
```

```python
# Qdrant collection: wiki_chunks
# vector: bge-m3 (1024d)
# payload: {doc_id, chunk_idx, text, category, source, url, fetched_at}
```

---

## 6. LLM 调度策略

| Agent / 任务 | 模型 | 缘由 |
|--------------|------|------|
| 规划师（Planner） | `claude-opus-4-8` | 一次性，影响全局结构；要求最高 |
| 章节作者（Author） | `claude-sonnet-4-6` | 主力写作，质量 vs 成本平衡 |
| 代码工程师（CodeEng） | `claude-sonnet-4-6` | 代码生成需要可靠性 |
| 质量校验（Validator） | `claude-sonnet-4-6` | 多维度判断，需要强推理 |
| 习题专家（Quiz） | `claude-haiku-4-5-20251001` | 题目结构化、模式化 |
| 信息收集摘要/去重 | `claude-haiku-4-5-20251001` | 量大、价廉 |
| 主题分类 / quiz 改写 | `claude-haiku-4-5-20251001` | 同上 |

**Prompt cache**：建课时把 Wiki 检索上下文放入 cache_control（5 分钟 TTL），同章节多 Agent、同任务多章节共享，预估降低 60-80% 输入 token。

成本预估（一门 8 章课）：
- 不优化：≈ $8-15
- 分层 + cache：≈ $1-3

---

## 7. 工程结构

```
MarsAgent/
├── apps/
│   ├── gateway/                       # Go (Gin)
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── api/       (handlers + SSE)
│   │   │   ├── stream/    (Redis Streams producer)
│   │   │   ├── grpcc/     (gRPC client → workers)
│   │   │   ├── sandbox/   (Docker scheduler)
│   │   │   ├── store/     (Postgres, MinIO 客户端)
│   │   │   └── config/
│   │   ├── go.mod
│   │   └── Dockerfile
│   ├── agents/                        # Python (FastAPI + LangGraph)
│   │   ├── marsagent/
│   │   │   ├── collector/  (5 source adapters)
│   │   │   ├── builder/    (5-Agent LangGraph DAG)
│   │   │   ├── llm/        (Anthropic 客户端 + 模型路由 + cache)
│   │   │   ├── rag/        (Qdrant + bge-m3 embed + rerank)
│   │   │   ├── stream/     (Redis Streams consumer)
│   │   │   ├── grpcs/      (gRPC server: WikiRetriever)
│   │   │   ├── storage/    (MD 文件 + Postgres ORM)
│   │   │   └── tasks.py
│   │   ├── pyproject.toml
│   │   └── Dockerfile
│   └── web/                           # React (Vite + TS)
│       ├── src/
│       │   ├── views/
│       │   │   ├── WikiBrowser/
│       │   │   ├── CourseBuilder/
│       │   │   └── CourseReader/
│       │   ├── components/  (shadcn/ui)
│       │   ├── lib/         (api client, sse hook)
│       │   └── routes.tsx
│       ├── package.json
│       └── Dockerfile
├── proto/wiki.proto
├── infra/
│   ├── docker-compose.yml
│   ├── postgres/init.sql
│   ├── qdrant/
│   └── sandbox-images/    (Dockerfile.python, Dockerfile.node, ...)
├── docs/superpowers/specs/
└── README.md
```

### 设计原则：模块边界

每个模块应能独立回答："做什么 / 怎么用 / 依赖什么"。具体边界：

- **Go gateway** 不持有任何 LLM 调用；只做 IO、调度、Stream 桥接、沙箱编排。
- **Python collector** 不持有建课逻辑；产出物只是 Wiki 条目。
- **Python builder** 只通过 gRPC `WikiRetriever` 读 Wiki，不直接读文件系统/向量库 ← 保证后续 Wiki 实现可替换。
- **proto/** 是契约，Go/Python 双端 codegen，避免 schema 漂移。
- **infra/sandbox-images/** 与 gateway 解耦，新增语言只加镜像 + 注册表项，不改 Go 代码。

---

## 8. 实施分期

每期 1-2 周可独立 demo。

### M1 — 骨架贯通
- docker-compose 起 Postgres / Qdrant / Redis / MinIO
- Go gateway: hello REST + SSE
- Python worker: 消费 Redis Streams 的 echo 任务并回传进度
- React Vite 起壳 + 三路由空页 + SSE 进度组件
- gRPC proto 定义 + 双向连通

**Demo**：前端点按钮 → 后端 5 秒后流式吐 `done`。

### M2 — Wiki 收集 MVP
- 接入 Tavily + arXiv + GitHub + Playwright（先 1 个博客源）
- Haiku 摘要 + simhash 去重 + bge-m3 切片 embedding → Qdrant
- MD 文件写 MinIO，元数据写 Postgres
- Wiki 浏览器视图：目录树 + 检索 + 渲染

**Demo**：`POST /api/wiki/collect {topic:"Transformer"}` → 浏览器看到 10+ 篇结构化条目。

### M3 — 建课 Agent MVP（裁剪 3 角色）
- 先做 Planner(Opus) + Author(Sonnet) + Validator(Sonnet)
- LangGraph DAG 跑通 + Prompt cache + 章节并行
- 课程产物存 MinIO，课程阅读器视图渲染（纯文本 + 代码块高亮）

**Demo**：输入"深度学习入门" → SSE 进度 → 8 章讲义出炉。

### M4 — 五角色完整版 + Docker 沙箱
- 补 CodeEng + Quiz 两个角色
- Go SandboxScheduler + Python/Node/Go base 镜像 + cgroup 限额
- 课程阅读器接 Monaco + `[Run]` 按钮 + Quiz 面板

**Demo**：课程内代码块可运行；quiz 可答题/查解析。

### M5 — 巡检 + 鲁棒性
- Redis Streams consumer-group 重试 + 死信队列
- Cron 巡检 + 增量更新（按 `url_hash` diff）
- 错误兜底、限流、可观测（slog/pino + OpenTelemetry traces）
- README + 一行 `docker-compose up` 启动文档

---

## 9. 风险与缓解

| 风险 | 缓解 |
|------|------|
| LLM 成本失控 | 模型分层 + Prompt cache + 任务级 token 预算上限 |
| 章节内容幻觉 | Validator 强制要求引用在 Wiki 中存在；不通过则重试 |
| 重复采集相同资料 | URL hash + content simhash 双层去重 |
| 沙箱逃逸 | `--network=none` + cgroup + read-only + non-root + 可选 gVisor；M5 加 fork bomb/网络逃逸测试集 |
| Docker socket 暴露给 Go 是攻击面 | 生产用 sysbox 或 remote docker host；MVP 接受风险 |
| 长任务（>30 min）中途崩溃丢进度 | Redis Streams 持久化 + consumer-group 重试；M5 加 LangGraph checkpoint 断点续跑 |
| 爬虫合规（robots.txt / 反爬 / API 配额） | 默认遵守 robots、UA 标识、域名级 rate-limit、官方 API 优先 |
| Qdrant 单机扩展性 | MVP 足够，规模化迁 HA cluster |
| 课程质量主观难量化 | M5 后建小规模评测集 + 人工 review 闭环 |

---

## 10. 测试策略

- **Go gateway**：handler 表驱动单测 + testcontainers 起 Postgres/Redis 做集成测试。
- **Python agents**：LLM 调用 record/replay 打桩 + LangGraph 节点单测 + 端到端 smoke（小主题真调一次 LLM，CI nightly）。
- **React 前端**：Vitest 组件测试 + Playwright e2e 跑三视图 happy path。
- **沙箱**：专门的安全测试用例集（fork bomb / 网络逃逸尝试 / 大文件写穿 / 时间炸弹）。

---

## 11. 范围外（明确不做）

- 用户系统 / 多租户 / 权限模型（M-后，接入外部底座）
- 课程版本管理 / 协同编辑
- 视频生成 / TTS
- 移动端原生 App
- 付费 / 计费
- 浏览器内 WASM 沙箱（已选 Docker 后端方案）

---

## 附录 A — 关键技术栈版本

| 类别 | 选型 |
|------|------|
| Go | 1.22+，Gin，go-redis/v9，pgx/v5，grpc-go，testcontainers-go |
| Python | 3.11+，FastAPI，LangGraph，anthropic-sdk，qdrant-client，sqlalchemy 2，redis.asyncio，grpcio |
| 前端 | Vite，React 18，TypeScript，Tailwind，shadcn/ui，TanStack Router，TanStack Query，react-markdown，Monaco Editor |
| 存储 | Postgres 16，Qdrant 1.x，MinIO，Redis 7 |
| Embedding | BAAI/bge-m3 (1024d) |
| 部署 | docker-compose（MVP）；后续可 K8s |

