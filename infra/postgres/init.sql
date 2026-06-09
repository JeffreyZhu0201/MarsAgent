-- M1 仅放占位 schema，M2/M3 会扩
create extension if not exists "uuid-ossp";

-- 预留 user_id 字段以支持后续多租户接入（spec §5.5）
-- M1 这些表只是定义骨架，gateway/agents 暂不读写
create table if not exists tasks (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid null,
  kind text not null,
  ref_id uuid,
  status text not null default 'pending',
  error text,
  created_at timestamptz not null default now()
);

-- M2: Wiki 知识库文档表
create table if not exists wiki_docs (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid null,
  slug text unique not null,
  category text not null default 'general',
  title text not null,
  url text not null,
  url_hash bytea not null,
  content_hash bytea not null,
  source text not null,
  quality_score real,
  language text,
  fetched_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  storage_path text not null,
  constraint unique_url_hash unique (url_hash)
);

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

-- M3: 课程表
create table if not exists courses (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid null,
  topic text not null,
  audience text,
  depth text,
  status text not null default 'pending',
  outline_json jsonb,
  storage_prefix text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
