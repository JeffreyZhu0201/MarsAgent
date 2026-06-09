# MarsAgent — 知识审核工作流 · 设计文档

- **日期**：2026-06-08
- **状态**：Draft v1 · 待评审
- **参考**：[LLM Wiki Agent](https://github.com/SamurAIGPT/llm-wiki-agent)

---

## 问题

当前信息收集 Agent 的 collector 在发现资料后立即写入 `wiki_docs` + `Qdrant`，没有人工确认环节。用户希望能：
1. 搜索 Agent 主动发现网络信息后**生成编辑草稿**
2. **人工预览 / 编辑 Markdown / 确认或拒绝**后再入库
3. 支持对已有知识条目进行**手动 CRUD**（增删查改、从空白创建）

---

## 架构

```
用户输入需求 (topic, sources, depth)
    │
    ▼
Phase 1: 搜索需求
    │
    ▼
Phase 2: Agent 搜索 → 多源采集 → AI 摘要 → 草稿
    │  (collector 写 drafts 表，不直接写 wiki_docs)
    │
    ▼
Phase 3: 人工审核台
    │    ├─ 预览内容 (Markdown + 引用)
    │    ├─ 编辑标题/正文/分类
    │    ├─ 确认 (approve) → Phase 4
    │    └─ 拒绝 (reject)
    │
    ▼
Phase 4: 发布
         ├─ 写 wiki_docs (Postgres)
         ├─ 写 Markdown 文件 (MinIO)
         ├─ 切片 + embedding → Qdrant
         ├─ 更新 drafts.status = published
         └─ 通知前端
```

---

## 表结构

### drafts（新增）

```sql
CREATE TABLE drafts (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  task_id       UUID NULL,
  user_id       UUID NULL,
  status        TEXT NOT NULL DEFAULT 'draft',
  title         TEXT NOT NULL,
  content_md    TEXT NOT NULL DEFAULT '',
  url           TEXT NOT NULL DEFAULT '',
  url_hash      BYTEA UNIQUE,
  source        TEXT NOT NULL DEFAULT '',
  category      TEXT NOT NULL DEFAULT 'general',
  revision      INT NOT NULL DEFAULT 1,
  summary       TEXT,
  quality_score REAL,
  language      TEXT DEFAULT 'en',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at  TIMESTAMPTZ,
  wiki_doc_id   UUID NULL REFERENCES wiki_docs(id) ON DELETE SET NULL
);
```

status 值: `draft` / `approved` / `rejected` / `published`

### wiki_docs（已有，增字段）

```sql
-- 已有表中新增
ALTER TABLE wiki_docs ADD COLUMN IF NOT EXISTS drafts_count INT NOT NULL DEFAULT 0;
```

---

## REST API

### 搜索触发

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/wiki/collect` | (已有) 触发搜索，draft_mode 默认开启 |

### 草稿 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/wiki/drafts` | 草稿列表（默认 draft 状态） |
| GET | `/api/wiki/drafts/:id` | 草稿详情 + Markdown 内容 |
| PUT | `/api/wiki/drafts/:id` | 编辑草稿标题/正文/分类 |
| POST | `/api/wiki/drafts` | 手动创建空白草稿 |
| DELETE | `/api/wiki/drafts/:id` | 删除草稿 |
| POST | `/api/wiki/drafts/:id/approve` | 确认 → 发布到 wiki_docs + MinIO + Qdrant |
| POST | `/api/wiki/drafts/:id/reject` | 拒绝 → 标记 rejected |

### Wiki 条目编辑

| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/wiki/docs/:slug` | 更新已发布条目标题/正文/分类 + 重新 embedding |
| DELETE | `/api/wiki/docs/:slug` | 删除已发布条目 |

---

## 三端文件变化

### Go Gateway

```
apps/gateway/internal/
├── store/
│   └── wiki.go         # 新建: Draft CRUD + wiki_doc update/delete
└── api/
    ├── router.go        # 注册新路由
    └── wiki.go          # 增加 draft/doc 处理函数
```

### Python Agent

```
apps/agents/marsagent/collector/tasks/collect.py
  → 写 drafts 表代替直接写 wiki_docs
  → 增强 AI 摘要为输出完整 Markdown 草稿
```

### React 前端

```
apps/web/src/views/
├── WikiBrowser.tsx        # 修改: 增加审核台面板、草稿列表
├── WikiDraftEditor.tsx    # 新建: 在线 Markdown 编辑器
└── WikiDocEditor.tsx      # 新建: 编辑/删除已发布条目

apps/web/src/lib/api.ts    # 新增 draft/doc CRUD 函数
```

---

## 前端 UI 变化

Wiki 浏览器从当前三栏布局升级为四面板：

1. **左侧** — 知识树切换：已发布 Wiki | 草稿列表 | 已拒绝
2. **中间** — 阅读/编辑区
3. **右侧** — 搜索 Agent | 发现信息 | RAG 诊断
4. **底部/浮动** — 操作栏：确认 / 编辑 / 删除 / 从空白新建

草稿条目在左侧用黄色 dot 标记，已发布用绿色 dot。

---

## 数据流详述

### 搜索 → 草稿

```
handle_collect()
  → 正常发现文档
  → 摘要 + 去重
  → 写入 drafts 表 (status='draft', content_md=AI 生成摘要)
  → 不写 MinIO, 不写 Qdrant, 不写 wiki_docs
  → 发 progress 事件: {stage: "draft_created", draft_id, title, url}
```

### 确认发布

```
POST /api/wiki/drafts/:id/approve
  → 读取 drafts 记录
  → 写入 wiki_docs (Postgres)
  → 写 Markdown 文件 (MinIO)
  → 切片 + embedding → Qdrant
  → 更新 drafts.status='published', wiki_doc_id, published_at
  → 返回 wiki_doc slug
```

### 编辑已发布条目

```
PUT /api/wiki/docs/:slug
  → 更新 wiki_docs.title / content
  → 重新写入 MinIO
  → 删除旧的 Qdrant points (prefixed doc_id)
  → 重新切片 + embedding → Qdrant
```

---

## 验收清单

- [ ] `collector` 改为 draft_mode，发现→写入 drafts 表
- [ ] `GET /api/wiki/drafts` 返回草稿列表
- [ ] `PUT /api/wiki/drafts/:id` 编辑草稿标题/正文/分类
- [ ] `POST /api/wiki/drafts/:id/approve` 发布到 wiki_docs + MinIO + Qdrant
- [ ] `POST /api/wiki/drafts/:id/reject` 拒绝草稿
- [ ] `POST /api/wiki/drafts` 从空白创建草稿
- [ ] `DELETE /api/wiki/drafts/:id` 删除草稿
- [ ] `PUT /api/wiki/docs/:slug` 编辑已发布条目 + 重新 embedding
- [ ] `DELETE /api/wiki/docs/:slug` 删除已发布条目
- [ ] `go test ./...`, `pytest`, `npm run build` 全部通过

---

## Self-Review

| 检查项 | 结果 |
|--------|------|
| Placeholder scan | 无 TBD/TODO 占位 |
| Internal consistency | 表/API/前端一致 |
| Scope check | 单一子系统（知识审核工作流），适合单个 spec |
| Ambiguity check | collector 的 draft_mode 有明确开关设计，不破坏现有无审核流程 |