-- +goose Up
CREATE TABLE collab_rooms (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  host_client_id TEXT NOT NULL,
  name TEXT,
  invite_token_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  default_role TEXT NOT NULL DEFAULT 'prompter',
  prompt_review_mode TEXT NOT NULL DEFAULT 'off',
  allow_maintainer_prompt_intercept INTEGER NOT NULL DEFAULT 1,
  allow_prompt_edit_before_approval INTEGER NOT NULL DEFAULT 1,
  require_host_approval_dangerous_tools INTEGER NOT NULL DEFAULT 1,
  auto_run_queue INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  revoked_at INTEGER
);

CREATE TABLE collab_participants (
  id TEXT PRIMARY KEY,
  room_id TEXT NOT NULL REFERENCES collab_rooms(id),
  client_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'prompter',
  status TEXT NOT NULL DEFAULT 'online',
  joined_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  UNIQUE(room_id, client_id)
);

CREATE TABLE collab_events (
  id TEXT PRIMARY KEY,
  room_id TEXT NOT NULL REFERENCES collab_rooms(id),
  session_id TEXT,
  actor_client_id TEXT,
  type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE prompt_queue (
  id TEXT PRIMARY KEY,
  room_id TEXT REFERENCES collab_rooms(id),
  session_id TEXT NOT NULL REFERENCES sessions(id),
  actor_client_id TEXT NOT NULL,
  content TEXT NOT NULL,
  original_content TEXT,
  reviewed_content TEXT,
  status TEXT NOT NULL DEFAULT 'submitted',
  requires_review INTEGER NOT NULL DEFAULT 0,
  reviewed_by_client_id TEXT,
  reviewed_at INTEGER,
  review_note TEXT,
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  completed_at INTEGER
);

CREATE TABLE idempotency_keys (
  key TEXT PRIMARY KEY,
  scope TEXT NOT NULL,
  result_json TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE session_run_locks (
  session_id TEXT PRIMARY KEY REFERENCES sessions(id),
  run_id TEXT NOT NULL,
  actor_client_id TEXT NOT NULL,
  started_at INTEGER NOT NULL
);

CREATE INDEX idx_collab_rooms_project ON collab_rooms(project_id);
CREATE INDEX idx_collab_participants_room ON collab_participants(room_id);
CREATE INDEX idx_collab_events_room_created ON collab_events(room_id, created_at ASC);
CREATE INDEX idx_prompt_queue_session_status ON prompt_queue(session_id, status);
CREATE INDEX idx_prompt_queue_room_position ON prompt_queue(room_id, position ASC);
CREATE INDEX idx_idempotency_keys_scope ON idempotency_keys(scope, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_idempotency_keys_scope;
DROP INDEX IF EXISTS idx_prompt_queue_room_position;
DROP INDEX IF EXISTS idx_prompt_queue_session_status;
DROP INDEX IF EXISTS idx_collab_events_room_created;
DROP INDEX IF EXISTS idx_collab_participants_room;
DROP INDEX IF EXISTS idx_collab_rooms_project;
DROP TABLE IF EXISTS session_run_locks;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS prompt_queue;
DROP TABLE IF EXISTS collab_events;
DROP TABLE IF EXISTS collab_participants;
DROP TABLE IF EXISTS collab_rooms;
