-- +goose Up
CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  title TEXT NOT NULL,
  model TEXT NOT NULL,
  agent TEXT NOT NULL,
  parent_id TEXT,
  snapshot_hash TEXT,
  token_input INTEGER DEFAULT 0,
  token_output INTEGER DEFAULT 0,
  cost REAL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  archived_at INTEGER
);

CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  role TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE parts (
  id TEXT PRIMARY KEY,
  message_id TEXT NOT NULL REFERENCES messages(id),
  type TEXT NOT NULL,
  order_num INTEGER NOT NULL,
  text TEXT,
  tool_name TEXT,
  tool_call_id TEXT,
  tool_args_json TEXT,
  tool_result_json TEXT,
  is_error INTEGER DEFAULT 0,
  duration_ms INTEGER,
  tokens_input INTEGER,
  tokens_output INTEGER,
  cost REAL,
  created_at INTEGER NOT NULL
);

CREATE TABLE config_kv (
  key TEXT NOT NULL,
  value_json TEXT NOT NULL,
  scope TEXT NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (key, scope)
);

CREATE INDEX idx_sessions_project_updated ON sessions(project_id, updated_at DESC);
CREATE INDEX idx_messages_session_created ON messages(session_id, created_at ASC);
CREATE INDEX idx_parts_message_order ON parts(message_id, order_num ASC);

-- +goose Down
DROP INDEX IF EXISTS idx_parts_message_order;
DROP INDEX IF EXISTS idx_messages_session_created;
DROP INDEX IF EXISTS idx_sessions_project_updated;
DROP TABLE IF EXISTS config_kv;
DROP TABLE IF EXISTS parts;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS projects;
