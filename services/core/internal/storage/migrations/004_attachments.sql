-- +goose Up
CREATE TABLE attachments (
  id            TEXT PRIMARY KEY,
  session_id    TEXT NOT NULL REFERENCES sessions(id),
  orig_name     TEXT NOT NULL,
  mime_type     TEXT NOT NULL,
  size_bytes    INTEGER NOT NULL,
  storage_path  TEXT NOT NULL,
  extracted     TEXT,
  is_chunked    INTEGER NOT NULL DEFAULT 0,
  chunk_count   INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL,
  deleted_at    INTEGER
);

CREATE INDEX idx_attachments_session ON attachments(session_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_attachments_session;
DROP TABLE IF EXISTS attachments;
