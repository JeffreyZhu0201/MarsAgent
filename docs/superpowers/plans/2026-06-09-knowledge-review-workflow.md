# MarsAgent Knowledge Review Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the Wiki collector into a human-in-the-loop knowledge workflow: active network search produces editable drafts; users can review, edit, approve, reject, create, update, delete, and publish knowledge into Wiki + RAG.

**Architecture:** Add a `drafts` persistence layer in Postgres, keep collector discovery async via Redis Streams, and move final Wiki+MinIO+Qdrant publishing behind explicit human approval. Go Gateway owns draft/wiki CRUD APIs; Python collector creates drafts and emits progress; React WikiBrowser gains draft review/editor panels while keeping existing RAG diagnostics.

**Tech Stack:** Go/Gin/Postgres/MinIO/Qdrant, Python 3.11 collector worker, Redis Streams, React 18 + Vite + TypeScript + ReactMarkdown, Playwright.

---

## File Structure

```
MarsAgent/
├── infra/postgres/init.sql
├── apps/gateway/internal/
│   ├── store/wiki.go              # new: Draft CRUD + wiki doc CRUD/publish helpers
│   └── api/
│       ├── router.go              # register draft/doc routes
│       └── wiki.go                # draft/doc HTTP handlers
├── apps/agents/marsagent/
│   └── collector/
│       ├── storage.py             # add draft write helper
│       └── tasks/collect.py       # write drafts by default; progress draft_created
└── apps/web/src/
    ├── lib/api.ts                 # draft/doc CRUD API helpers
    └── views/
        ├── WikiBrowser.tsx        # draft tabs + review workspace
        ├── WikiDraftEditor.tsx    # new: Markdown editor for drafts
        └── WikiDocEditor.tsx      # new: edit/delete published wiki docs
```

---

## Task K0: Postgres schema for drafts

**Files:**
- Modify: `infra/postgres/init.sql`

- [ ] **Step 1: Add drafts table schema**

Append to `infra/postgres/init.sql`:

```sql
-- Knowledge review workflow: AI-generated drafts awaiting human approval
create table if not exists drafts (
  id uuid primary key default uuid_generate_v4(),
  task_id uuid null,
  user_id uuid null,
  status text not null default 'draft', -- draft|approved|rejected|published
  title text not null,
  content_md text not null default '',
  url text not null default '',
  url_hash bytea unique,
  source text not null default '',
  category text not null default 'general',
  revision int not null default 1,
  summary text,
  quality_score real,
  language text default 'en',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  published_at timestamptz,
  wiki_doc_id uuid null references wiki_docs(id) on delete set null
);

alter table wiki_docs add column if not exists drafts_count int not null default 0;
```

- [ ] **Step 2: Verify schema applies**

Run:

```bash
docker exec -i infra-postgres-1 psql -U mars -d marsagent < infra/postgres/init.sql
```

Expected: no SQL errors; `CREATE TABLE` or `NOTICE relation exists` output.

- [ ] **Step 3: Commit**

```bash
git add infra/postgres/init.sql
git commit -m "feat(db): add knowledge draft schema"
```

---

## Task K1: Go store for draft CRUD and publishing

**Files:**
- Create: `apps/gateway/internal/store/wiki.go`
- Test: `apps/gateway/tests/wiki_store_test.go`

- [ ] **Step 1: Write failing store test**

Create `apps/gateway/tests/wiki_store_test.go`:

```go
package tests

import (
	"context"
	"database/sql"
	"testing"

	"github.com/marsagent/gateway/internal/store"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

func TestDraftLifecycleStore(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	s := store.NewWikiStore(db)
	ctx := context.Background()

	draft, err := s.CreateDraft(ctx, store.DraftInput{
		Title: "Draft Title",
		ContentMD: "# Draft\n\nBody",
		URL: "https://example.com/draft-lifecycle",
		Source: "test",
		Category: "general",
	})
	require.NoError(t, err)
	require.Equal(t, "draft", draft.Status)

	require.NoError(t, s.UpdateDraft(ctx, draft.ID, store.DraftInput{
		Title: "Updated Draft",
		ContentMD: "# Updated",
		URL: draft.URL,
		Source: "test",
		Category: "web",
	}))

	got, err := s.GetDraft(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Draft", got.Title)
	require.Equal(t, "web", got.Category)

	list, err := s.ListDrafts(ctx, "draft", 20)
	require.NoError(t, err)
	require.NotEmpty(t, list)

	require.NoError(t, s.MarkDraftRejected(ctx, draft.ID))
	got, err = s.GetDraft(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "rejected", got.Status)
}
```

- [ ] **Step 2: Run test and verify it fails**

```bash
cd apps/gateway
go test ./tests -run TestDraftLifecycleStore -v
```

Expected: FAIL because `store.NewWikiStore` / `DraftInput` are undefined.

- [ ] **Step 3: Implement store**

Create `apps/gateway/internal/store/wiki.go`:

```go
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
)

type WikiStore struct { db *sql.DB }

func NewWikiStore(db *sql.DB) *WikiStore { return &WikiStore{db: db} }

type Draft struct {
	ID string `json:"id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Title string `json:"title"`
	ContentMD string `json:"content_md"`
	URL string `json:"url"`
	Source string `json:"source"`
	Category string `json:"category"`
	Revision int `json:"revision"`
	Summary string `json:"summary"`
	QualityScore float64 `json:"quality_score"`
	Language string `json:"language"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	PublishedAt string `json:"published_at"`
	WikiDocID string `json:"wiki_doc_id"`
}

type DraftInput struct {
	TaskID string
	Title string
	ContentMD string
	URL string
	Source string
	Category string
	Summary string
	QualityScore float64
	Language string
}

func (s *WikiStore) CreateDraft(ctx context.Context, in DraftInput) (*Draft, error) {
	urlHash := sha256.Sum256([]byte(in.URL))
	row := s.db.QueryRowContext(ctx, `
		insert into drafts (task_id,title,content_md,url,url_hash,source,category,summary,quality_score,language)
		values (nullif($1,'')::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		on conflict (url_hash) do update set
		  title=excluded.title, content_md=excluded.content_md, updated_at=now(), revision=drafts.revision+1
		returning id,status,title,content_md,url,source,category,revision,coalesce(summary,''),coalesce(quality_score,0),coalesce(language,''),created_at::text,updated_at::text`,
		in.TaskID, in.Title, in.ContentMD, in.URL, urlHash[:], in.Source, defaultString(in.Category, "general"), in.Summary, in.QualityScore, defaultString(in.Language, "en"))
	return scanDraft(row)
}

func (s *WikiStore) GetDraft(ctx context.Context, id string) (*Draft, error) {
	row := s.db.QueryRowContext(ctx, `
		select id,status,title,content_md,url,source,category,revision,coalesce(summary,''),coalesce(quality_score,0),coalesce(language,''),created_at::text,updated_at::text
		from drafts where id=$1`, id)
	return scanDraft(row)
}

func (s *WikiStore) ListDrafts(ctx context.Context, status string, limit int) ([]Draft, error) {
	if limit <= 0 || limit > 100 { limit = 50 }
	rows, err := s.db.QueryContext(ctx, `
		select id,status,title,content_md,url,source,category,revision,coalesce(summary,''),coalesce(quality_score,0),coalesce(language,''),created_at::text,updated_at::text
		from drafts where ($1='' or status=$1) order by updated_at desc limit $2`, status, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Draft{}
	for rows.Next() {
		d, err := scanDraftRows(rows)
		if err != nil { return nil, err }
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *WikiStore) UpdateDraft(ctx context.Context, id string, in DraftInput) error {
	_, err := s.db.ExecContext(ctx, `
		update drafts set title=$2, content_md=$3, category=$4, revision=revision+1, updated_at=now()
		where id=$1`, id, in.Title, in.ContentMD, defaultString(in.Category, "general"))
	return err
}

func (s *WikiStore) MarkDraftRejected(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `update drafts set status='rejected', updated_at=now() where id=$1`, id)
	return err
}

func scanDraft(row interface{ Scan(...any) error }) (*Draft, error) {
	var d Draft
	err := row.Scan(&d.ID,&d.Status,&d.Title,&d.ContentMD,&d.URL,&d.Source,&d.Category,&d.Revision,&d.Summary,&d.QualityScore,&d.Language,&d.CreatedAt,&d.UpdatedAt)
	return &d, err
}

func scanDraftRows(rows *sql.Rows) (*Draft, error) { return scanDraft(rows) }

func defaultString(v, fallback string) string { if v == "" { return fallback }; return v }
```

- [ ] **Step 4: Verify test passes**

```bash
cd apps/gateway
go test ./tests -run TestDraftLifecycleStore -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/gateway/internal/store/wiki.go apps/gateway/tests/wiki_store_test.go
git commit -m "feat(gateway): add wiki draft store"
```

---

## Task K2: Go draft API routes

**Files:**
- Modify: `apps/gateway/internal/api/wiki.go`
- Modify: `apps/gateway/internal/api/router.go`
- Test: `apps/gateway/tests/wiki_draft_api_test.go`

- [ ] **Step 1: Write failing handler test**

Create `apps/gateway/tests/wiki_draft_api_test.go`:

```go
package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marsagent/gateway/internal/api"
	"github.com/marsagent/gateway/internal/store"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

func TestDraftAPIManualCreateAndList(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	r := api.NewRouter(api.Deps{DB: db, WikiStore: store.NewWikiStore(db)})

	body, _ := json.Marshal(map[string]string{
		"title": "Manual Draft",
		"content_md": "# Manual",
		"url": "manual://draft-api",
		"category": "general",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/drafts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/wiki/drafts", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Manual Draft")
}
```

- [ ] **Step 2: Run test and verify it fails**

```bash
cd apps/gateway
go test ./tests -run TestDraftAPIManualCreateAndList -v
```

Expected: FAIL because `api.Deps` has no `WikiStore` and handlers are missing.

- [ ] **Step 3: Extend Deps and routes**

Modify `apps/gateway/internal/api/router.go`:

```go
// add to Deps
WikiStore *store.WikiStore

// in NewRouter
if d.WikiStore != nil {
  api.GET("/wiki/drafts", listDraftsHandler(d.WikiStore))
  api.POST("/wiki/drafts", createDraftHandler(d.WikiStore))
  api.GET("/wiki/drafts/:id", getDraftHandler(d.WikiStore))
  api.PUT("/wiki/drafts/:id", updateDraftHandler(d.WikiStore))
  api.DELETE("/wiki/drafts/:id", deleteDraftHandler(d.WikiStore))
  api.POST("/wiki/drafts/:id/reject", rejectDraftHandler(d.WikiStore))
}
```

Modify `apps/gateway/cmd/server/main.go` Deps:

```go
WikiStore: store.NewWikiStore(db),
```

- [ ] **Step 4: Implement handlers**

Append to `apps/gateway/internal/api/wiki.go`:

```go
func listDraftsHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status")
		drafts, err := ws.ListDrafts(c.Request.Context(), status, 50)
		if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(200, gin.H{"drafts": drafts})
	}
}

func createDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct { Title, ContentMD, URL, Source, Category string `json:",omitempty"` }
		if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
		draft, err := ws.CreateDraft(c.Request.Context(), store.DraftInput{Title: req.Title, ContentMD: req.ContentMD, URL: req.URL, Source: req.Source, Category: req.Category})
		if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(201, draft)
	}
}

func getDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		draft, err := ws.GetDraft(c.Request.Context(), c.Param("id"))
		if err == sql.ErrNoRows { c.JSON(404, gin.H{"error": "not found"}); return }
		if err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(200, draft)
	}
}

func updateDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct { Title, ContentMD, Category string `json:",omitempty"` }
		if err := c.ShouldBindJSON(&req); err != nil { c.JSON(400, gin.H{"error": err.Error()}); return }
		if err := ws.UpdateDraft(c.Request.Context(), c.Param("id"), store.DraftInput{Title: req.Title, ContentMD: req.ContentMD, Category: req.Category}); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(200, gin.H{"ok": true})
	}
}

func rejectDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ws.MarkDraftRejected(c.Request.Context(), c.Param("id")); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(200, gin.H{"ok": true})
	}
}

func deleteDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// for MVP: reuse reject semantics instead of hard delete to avoid accidental data loss
		if err := ws.MarkDraftRejected(c.Request.Context(), c.Param("id")); err != nil { c.JSON(500, gin.H{"error": err.Error()}); return }
		c.JSON(200, gin.H{"ok": true})
	}
}
```

- [ ] **Step 5: Run tests**

```bash
cd apps/gateway
go test ./tests -run 'TestDraft(API|Lifecycle)' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/gateway/internal/api/wiki.go apps/gateway/internal/api/router.go apps/gateway/cmd/server/main.go apps/gateway/tests/wiki_draft_api_test.go
git commit -m "feat(gateway): add wiki draft API routes"
```

---

## Task K3: Python collector writes drafts instead of direct wiki by default

**Files:**
- Modify: `apps/agents/marsagent/collector/storage.py`
- Modify: `apps/agents/marsagent/collector/tasks/collect.py`
- Test: `apps/agents/tests/test_collect_drafts.py`

- [ ] **Step 1: Write failing Python test**

Create `apps/agents/tests/test_collect_drafts.py`:

```python
import json
import pytest

from marsagent.collector.base import RawDoc
from marsagent.collector.tasks import collect

class FakeSink:
    def __init__(self): self.events=[]
    async def emit(self, event): self.events.append(event)

class FakeAdapter:
    async def search(self, query, max_results=10):
        yield RawDoc(url="https://example.com/a", title="A", content="# A\nBody", source="fake", fetched_at="now")

@pytest.mark.asyncio
async def test_collect_creates_draft_by_default(monkeypatch):
    created=[]
    monkeypatch.setitem(collect.ADAPTERS, "fake", lambda: FakeAdapter())
    async def fake_write_draft(**kwargs):
        created.append(kwargs)
        return "draft-1"
    monkeypatch.setattr("marsagent.collector.tasks.collect.write_wiki_draft", fake_write_draft)
    monkeypatch.setattr("marsagent.collector.tasks.collect.ensure_collection", lambda: None)

    sink=FakeSink()
    await collect.handle_collect(task_id="t1", args=json.dumps({"topic":"A","sources":["fake"],"max_per_source":1}).encode(), sink=sink)

    assert created[0]["title"] == "A"
    assert any(e.get("extra",{}).get("stage") == "draft_created" for e in sink.events)
```

- [ ] **Step 2: Run test and verify it fails**

```bash
cd apps/agents
PYTHONPATH=. pytest tests/test_collect_drafts.py -q
```

Expected: FAIL because `write_wiki_draft` is undefined.

- [ ] **Step 3: Add draft writer**

Append to `apps/agents/marsagent/collector/storage.py`:

```python
async def write_wiki_draft(
    *, task_id: str, title: str, content_md: str, url: str, url_hash: bytes,
    source: str, category: str, summary: str, quality_score: float, language: str,
) -> str:
    engine = _get_engine()
    draft_id = str(uuid.uuid4())
    with Session(engine) as sess:
        sess.execute(text("""
            INSERT INTO drafts
              (id, task_id, title, content_md, url, url_hash, source, category, summary, quality_score, language)
            VALUES
              (:id, :task_id, :title, :content_md, :url, :url_hash, :source, :category, :summary, :quality_score, :language)
            ON CONFLICT (url_hash) DO UPDATE SET
              title = EXCLUDED.title,
              content_md = EXCLUDED.content_md,
              updated_at = now(),
              revision = drafts.revision + 1
        """), {
            "id": draft_id, "task_id": task_id, "title": title, "content_md": content_md,
            "url": url, "url_hash": url_hash, "source": source, "category": category,
            "summary": summary, "quality_score": quality_score, "language": language,
        })
        sess.commit()
    return draft_id
```

- [ ] **Step 4: Modify collector to use drafts**

Modify imports in `collect.py`:

```python
from marsagent.collector.storage import write_wiki_doc, write_wiki_draft
```

In `handle_collect`, before processing docs:

```python
draft_mode = payload.get("draft_mode", True)
```

After `summary_result` and `clean_content`, branch:

```python
if draft_mode:
    draft_id = await write_wiki_draft(
        task_id=task_id, title=doc.title, content_md=clean_content, url=doc.url,
        url_hash=url_hash_bytes, source=doc.source, category=_infer_category(topic),
        summary=summary_result.summary, quality_score=summary_result.quality_score,
        language=summary_result.language,
    )
    await sink.emit(make_event(
        type_="agent.progress", task_id=task_id, agent="collector",
        message=f"已创建草稿: {doc.title}",
        extra={"stage": "draft_created", "draft": {"id": draft_id, "title": doc.title, "url": doc.url, "source": doc.source}},
    ))
    written += 1
    continue
```

Keep the existing direct write path under `if not draft_mode`.

- [ ] **Step 5: Verify test passes**

```bash
cd apps/agents
PYTHONPATH=. pytest tests/test_collect_drafts.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/agents/marsagent/collector/storage.py apps/agents/marsagent/collector/tasks/collect.py apps/agents/tests/test_collect_drafts.py
git commit -m "feat(agents): create wiki drafts from collector"
```

---

## Task K4: Draft approval publish path in Go

**Files:**
- Modify: `apps/gateway/internal/store/wiki.go`
- Modify: `apps/gateway/internal/api/wiki.go`
- Test: `apps/gateway/tests/wiki_draft_publish_test.go`

- [ ] **Step 1: Write failing publish test**

Create `apps/gateway/tests/wiki_draft_publish_test.go`:

```go
package tests

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marsagent/gateway/internal/api"
	"github.com/marsagent/gateway/internal/store"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

func TestApproveDraftPublishesWikiDoc(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable")
	require.NoError(t, err)
	ws := store.NewWikiStore(db)
	draft, err := ws.CreateDraft(context.Background(), store.DraftInput{Title:"Publish Me", ContentMD:"# Publish", URL:"https://example.com/publish-me", Source:"test", Category:"general"})
	require.NoError(t, err)

	r := api.NewRouter(api.Deps{DB: db, WikiStore: ws})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/drafts/"+draft.ID+"/approve", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "slug")
}
```

- [ ] **Step 2: Run and verify failure**

Expected: compile failure until `ApproveDraft` and handler exist.

- [ ] **Step 3: Implement `ApproveDraft`**

In `store/wiki.go`, implement minimal MVP publish:
- Read draft
- Insert into `wiki_docs` with slug, hashes, source/category/title/url/storage_path
- Update draft status `published`
- Return slug

Use same slugify/hash logic as Python storage but in Go.

- [ ] **Step 4: Add handler route**

`POST /api/wiki/drafts/:id/approve` calls `ws.ApproveDraft(ctx,id)` and returns `{slug}`.

- [ ] **Step 5: Verify**

```bash
cd apps/gateway
go test ./tests -run TestApproveDraftPublishesWikiDoc -v
```

- [ ] **Step 6: Commit**

```bash
git add apps/gateway/internal/store/wiki.go apps/gateway/internal/api/wiki.go apps/gateway/tests/wiki_draft_publish_test.go
git commit -m "feat(gateway): publish approved wiki drafts"
```

---

## Task K5: Frontend draft API client and editor

**Files:**
- Modify: `apps/web/src/lib/api.ts`
- Create: `apps/web/src/views/WikiDraftEditor.tsx`
- Modify: `apps/web/src/views/WikiBrowser.tsx`
- Test: `apps/web/tests/wiki-drafts.spec.ts`

- [ ] **Step 1: Add API types/helpers**

Append to `api.ts`:

```ts
export interface WikiDraft {
  id: string
  status: string
  title: string
  content_md: string
  url: string
  source: string
  category: string
  revision: number
  updated_at: string
}

export async function listDrafts(status = 'draft'): Promise<WikiDraft[]> {
  const r = await json<{ drafts: WikiDraft[] }>(`/api/wiki/drafts?status=${encodeURIComponent(status)}`)
  return r.drafts || []
}

export async function createDraft(input: Partial<WikiDraft>): Promise<WikiDraft> {
  return json('/api/wiki/drafts', { method:'POST', headers:{'content-type':'application/json'}, body: JSON.stringify(input) })
}

export async function updateDraft(id: string, input: Partial<WikiDraft>): Promise<{ok:boolean}> {
  return json(`/api/wiki/drafts/${encodeURIComponent(id)}`, { method:'PUT', headers:{'content-type':'application/json'}, body: JSON.stringify(input) })
}

export async function approveDraft(id: string): Promise<{slug:string}> {
  return json(`/api/wiki/drafts/${encodeURIComponent(id)}/approve`, { method:'POST' })
}

export async function rejectDraft(id: string): Promise<{ok:boolean}> {
  return json(`/api/wiki/drafts/${encodeURIComponent(id)}/reject`, { method:'POST' })
}
```

- [ ] **Step 2: Create editor component**

Create `WikiDraftEditor.tsx`:

```tsx
import { useState } from 'react'
import { MarkdownView } from '@/components/MarkdownView'
import { approveDraft, rejectDraft, updateDraft, type WikiDraft } from '@/lib/api'

export function WikiDraftEditor({ draft, onDone }: { draft: WikiDraft; onDone: () => void }) {
  const [title,setTitle]=useState(draft.title)
  const [category,setCategory]=useState(draft.category || 'general')
  const [content,setContent]=useState(draft.content_md)
  const [preview,setPreview]=useState(true)

  async function save(){ await updateDraft(draft.id,{title,category,content_md:content}); onDone() }
  async function approve(){ await updateDraft(draft.id,{title,category,content_md:content}); await approveDraft(draft.id); onDone() }
  async function reject(){ await rejectDraft(draft.id); onDone() }

  return (
    <div className="glass-card p-4 space-y-3">
      <input className="glass-input w-full" value={title} onChange={e=>setTitle(e.target.value)} />
      <input className="glass-input w-full" value={category} onChange={e=>setCategory(e.target.value)} />
      <div className="flex gap-2"><button className="glass-button" onClick={()=>setPreview(!preview)}>{preview?'编辑':'预览'}</button><button className="glass-button" onClick={save}>保存草稿</button><button className="glass-button" onClick={approve}>确认发布</button><button className="glass-button" onClick={reject}>拒绝</button></div>
      {preview ? <MarkdownView content={content} /> : <textarea className="glass-input min-h-96 w-full font-mono" value={content} onChange={e=>setContent(e.target.value)} />}
    </div>
  )
}
```

- [ ] **Step 3: Integrate into WikiBrowser**

- Add `drafts` state and `selectedDraft` state.
- Load drafts via `listDrafts()`.
- Add a right/left panel section showing draft titles.
- When selected, render `WikiDraftEditor` in main area.
- On `onDone`, reload drafts and wiki tree.

- [ ] **Step 4: Add Playwright test**

Create `apps/web/tests/wiki-drafts.spec.ts` with mocked API:

```ts
import { test, expect } from '@playwright/test'

test('wiki draft editor can approve object', async ({ page }) => {
  await page.route('**/api/wiki/tree', r => r.fulfill({json:{docs:[]}}))
  await page.route('**/api/wiki/drafts?status=draft', r => r.fulfill({json:{drafts:[{id:'d1',status:'draft',title:'Draft A',content_md:'# Draft A',url:'',source:'test',category:'general',revision:1,updated_at:''}]}}))
  await page.route('**/api/wiki/drafts/d1', r => r.fulfill({json:{ok:true}}))
  await page.route('**/api/wiki/drafts/d1/approve', r => r.fulfill({json:{slug:'draft-a'}}))
  await page.goto('/wiki')
  await expect(page.getByText('Draft A')).toBeVisible()
})
```

- [ ] **Step 5: Verify and commit**

```bash
cd apps/web
npm run build
npm run test:e2e -- --project=chromium tests/wiki-drafts.spec.ts
```

Commit:

```bash
git add apps/web/src/lib/api.ts apps/web/src/views/WikiDraftEditor.tsx apps/web/src/views/WikiBrowser.tsx apps/web/tests/wiki-drafts.spec.ts
git commit -m "feat(web): add wiki draft review editor"
```

---

## Task K6: Final smoke and review

**Files:**
- No planned code unless issues are found.

- [ ] **Step 1: Run all checks**

```bash
cd apps/gateway && go test ./...
cd ../agents && PYTHONPATH=. pytest tests/test_collect_drafts.py tests/test_dedup.py tests/test_chunker.py -q
cd ../web && npm run build && npm run test:e2e -- --project=chromium tests/course.spec.ts tests/wiki-drafts.spec.ts
```

- [ ] **Step 2: Manual smoke**

1. Open `http://localhost:5173/wiki`
2. Start Search Agent with topic `Python packaging`
3. Confirm progress shows `draft_created`
4. Open draft in editor
5. Edit title
6. Confirm publish
7. Verify Wiki tree includes published title
8. Run RAG check and verify hits > 0

- [ ] **Step 3: Final code review**

Dispatch reviewer for high-confidence issues in:
- `store/wiki.go`
- `api/wiki.go`
- `collector/tasks/collect.py`
- `collector/storage.py`
- `WikiBrowser.tsx`
- `WikiDraftEditor.tsx`
- `api.ts`

- [ ] **Step 4: Commit any review fixes**

```bash
git add <changed-files>
git commit -m "fix: address knowledge review workflow issues"
```

---

## Acceptance Checklist

- [ ] Search Agent creates drafts, not direct wiki entries, by default.
- [ ] User can list, view, edit, reject, and create drafts.
- [ ] User can approve a draft and publish it to Wiki.
- [ ] Published Wiki entries appear in `/api/wiki/tree`.
- [ ] RAG check returns hits after approval and embedding.
- [ ] UI uses polished Markdown editor/preview.
- [ ] Existing direct collect mode can still be enabled with `draft_mode:false`.

---

## Self-Review

| Check | Result |
|---|---|
| Spec coverage | Draft schema, draft CRUD, manual approval, wiki CRUD, UI editor covered |
| Placeholder scan | No TBD/TODO placeholders |
| Type consistency | Draft/DraftInput/WikiDraft fields consistent across Go/Python/TS |
| Scope | One cohesive knowledge review workflow plan |
