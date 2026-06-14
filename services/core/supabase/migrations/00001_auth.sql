-- Postgres-flavored mirror of services/core/internal/storage/migrations/003_auth.sql
-- Apply to Supabase via either:
--   supabase db push                           (after `supabase link`)
--   psql "$ZERO_SUPABASE_DB_URL" -f 00001_auth.sql
--
-- Schema is intentionally identical to the SQLite tables so SupabaseAuthStore
-- and storage.DB scan the same fields. Timestamps are bigint (Unix epoch ms)
-- to keep parity with the Go client code.

create table if not exists public.users (
  id            text primary key,
  google_id     text not null unique,
  email         text not null,
  display_name  text not null default '',
  avatar_url    text not null default '',
  role          text not null default 'user',
  created_at    bigint not null,
  updated_at    bigint not null
);

create table if not exists public.auth_sessions (
  id          text primary key,
  user_id     text not null references public.users(id) on delete cascade,
  expires_at  bigint not null,
  created_at  bigint not null
);

create index if not exists idx_users_email           on public.users(email);
create index if not exists idx_users_google_id       on public.users(google_id);
create index if not exists idx_auth_sessions_user    on public.auth_sessions(user_id);
create index if not exists idx_auth_sessions_expires on public.auth_sessions(expires_at);

-- RLS hardening: the daemon connects as the postgres role with
-- ZERO_SUPABASE_DB_URL, which bypasses RLS by design. We still enable RLS so
-- that any future PostgREST/anon-key access is denied by default.
alter table public.users          enable row level security;
alter table public.auth_sessions  enable row level security;
