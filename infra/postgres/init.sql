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

-- M4: Online Judge
create table if not exists problems (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid null,
  title text not null,
  description_md text not null default '',
  tags text[] not null default '{}',
  difficulty text not null default 'medium',
  time_limit_ms int not null default 2000,
  memory_limit_mb int not null default 256,
  visible bool not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists test_cases (
  id uuid primary key default uuid_generate_v4(),
  problem_id uuid not null references problems(id) on delete cascade,
  input text not null default '',
  expected_output text not null,
  is_sample bool not null default false,
  is_hidden bool not null default true,
  score int not null default 100,
  ordering int not null default 0,
  unique (problem_id, ordering)
);

create table if not exists submissions (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid null,
  problem_id uuid not null references problems(id),
  code text not null,
  lang text not null,
  status text not null default 'pending',
  score int not null default 0,
  duration_ms int not null default 0,
  memory_kb int not null default 0,
  error_msg text,
  created_at timestamptz not null default now()
);

create table if not exists submission_results (
  id uuid primary key default uuid_generate_v4(),
  submission_id uuid not null references submissions(id) on delete cascade,
  test_case_id uuid not null references test_cases(id),
  status text not null default 'pending',
  actual_output text,
  duration_ms int not null default 0,
  memory_kb int not null default 0,
  score int not null default 0
);

create index if not exists idx_submissions_problem_id on submissions(problem_id);
create index if not exists idx_submissions_user_id on submissions(user_id);
create index if not exists idx_submissions_created_at on submissions(created_at desc);
create index if not exists idx_test_cases_problem_id on test_cases(problem_id);
create index if not exists idx_submission_results_submission_id on submission_results(submission_id);

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
