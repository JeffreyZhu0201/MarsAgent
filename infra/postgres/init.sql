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
