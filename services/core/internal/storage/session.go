package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type Session struct {
	ID           string  `json:"id"`
	ProjectID    string  `json:"projectId"`
	Title        string  `json:"title"`
	Model        string  `json:"model"`
	Agent        string  `json:"agent"`
	ParentID     *string `json:"parentId,omitempty"`
	SnapshotHash *string `json:"snapshotHash,omitempty"`
	TokenInput   int64   `json:"tokenInput"`
	TokenOutput  int64   `json:"tokenOutput"`
	Cost         float64 `json:"cost"`
	CreatedAt    int64   `json:"createdAt"`
	UpdatedAt    int64   `json:"updatedAt"`
	ArchivedAt   *int64  `json:"archivedAt,omitempty"`
}

type Message struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type Part struct {
	ID             string   `json:"id"`
	MessageID      string   `json:"messageId"`
	Type           string   `json:"type"`
	OrderNum       int64    `json:"orderNum"`
	Text           *string  `json:"text,omitempty"`
	ToolName       *string  `json:"toolName,omitempty"`
	ToolCallID     *string  `json:"toolCallId,omitempty"`
	ToolArgsJSON   *string  `json:"toolArgsJson,omitempty"`
	ToolResultJSON *string  `json:"toolResultJson,omitempty"`
	IsError        bool     `json:"isError"`
	DurationMS     *int64   `json:"durationMs,omitempty"`
	TokensInput    *int64   `json:"tokensInput,omitempty"`
	TokensOutput   *int64   `json:"tokensOutput,omitempty"`
	Cost           *float64 `json:"cost,omitempty"`
	CreatedAt      int64    `json:"createdAt"`
}

type CreateSessionInput struct {
	ProjectID string
	Title     string
	Model     string
	Agent     string
	ParentID  *string
}

func (db *DB) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	now := time.Now().UnixMilli()
	s := &Session{
		ID:        uuid.New().String(),
		ProjectID: input.ProjectID,
		Title:     input.Title,
		Model:     input.Model,
		Agent:     input.Agent,
		ParentID:  input.ParentID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := db.conn.ExecContext(ctx, `INSERT INTO sessions (id, project_id, title, model, agent, parent_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, s.ID, s.ProjectID, s.Title, s.Model, s.Agent, nullableString(s.ParentID), s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (db *DB) GetSession(ctx context.Context, id string) (*Session, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT id, project_id, title, model, agent, parent_id, snapshot_hash, token_input, token_output, cost, created_at, updated_at, archived_at FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (db *DB) ListSessions(ctx context.Context, projectID string) ([]Session, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, project_id, title, model, agent, parent_id, snapshot_hash, token_input, token_output, cost, created_at, updated_at, archived_at FROM sessions WHERE project_id = ? AND archived_at IS NULL ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		s, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}

func (db *DB) UpdateSession(ctx context.Context, id, title string) (*Session, error) {
	return db.UpdateSessionFields(ctx, id, SessionPatch{Title: &title})
}

// SessionPatch is a partial update set; nil fields are ignored.
type SessionPatch struct {
	Title *string
	Model *string
	Agent *string
}

// UpdateSessionFields applies any non-nil fields in the patch and returns the
// refreshed session. At least one field must be non-nil.
func (db *DB) UpdateSessionFields(ctx context.Context, id string, patch SessionPatch) (*Session, error) {
	sets := []string{}
	args := []any{}
	if patch.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *patch.Title)
	}
	if patch.Model != nil {
		sets = append(sets, "model = ?")
		args = append(args, *patch.Model)
	}
	if patch.Agent != nil {
		sets = append(sets, "agent = ?")
		args = append(args, *patch.Agent)
	}
	if len(sets) == 0 {
		return nil, errors.New("no fields to update")
	}
	now := time.Now().UnixMilli()
	sets = append(sets, "updated_at = ?")
	args = append(args, now)
	args = append(args, id)

	query := "UPDATE sessions SET " + joinComma(sets) + " WHERE id = ? AND archived_at IS NULL"
	res, err := db.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return db.GetSession(ctx, id)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func (db *DB) ArchiveSession(ctx context.Context, id string) error {
	now := time.Now().UnixMilli()
	res, err := db.conn.ExecContext(ctx, `UPDATE sessions SET archived_at = ?, updated_at = ? WHERE id = ? AND archived_at IS NULL`, now, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) CreateMessage(ctx context.Context, sessionID, role string) (*Message, error) {
	now := time.Now().UnixMilli()
	m := &Message{ID: uuid.New().String(), SessionID: sessionID, Role: role, CreatedAt: now, UpdatedAt: now}
	_, err := db.conn.ExecContext(ctx, `INSERT INTO messages (id, session_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, m.ID, m.SessionID, m.Role, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (db *DB) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, session_id, role, created_at, updated_at FROM messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (db *DB) DeleteMessage(ctx context.Context, id string) error {
	if _, err := db.conn.ExecContext(ctx, `DELETE FROM parts WHERE message_id = ?`, id); err != nil {
		return err
	}
	res, err := db.conn.ExecContext(ctx, `DELETE FROM messages WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type CreatePartInput struct {
	MessageID      string
	Type           string
	OrderNum       int64
	Text           *string
	ToolName       *string
	ToolCallID     *string
	ToolArgsJSON   *string
	ToolResultJSON *string
	IsError        bool
}

func (db *DB) CreatePart(ctx context.Context, input CreatePartInput) (*Part, error) {
	p := &Part{ID: uuid.New().String(), MessageID: input.MessageID, Type: input.Type, OrderNum: input.OrderNum, Text: input.Text, ToolName: input.ToolName, ToolCallID: input.ToolCallID, ToolArgsJSON: input.ToolArgsJSON, ToolResultJSON: input.ToolResultJSON, IsError: input.IsError, CreatedAt: time.Now().UnixMilli()}
	_, err := db.conn.ExecContext(ctx, `INSERT INTO parts (id, message_id, type, order_num, text, tool_name, tool_call_id, tool_args_json, tool_result_json, is_error, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.ID, p.MessageID, p.Type, p.OrderNum, nullableString(p.Text), nullableString(p.ToolName), nullableString(p.ToolCallID), nullableString(p.ToolArgsJSON), nullableString(p.ToolResultJSON), boolToInt(p.IsError), p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) ListParts(ctx context.Context, messageID string) ([]Part, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, message_id, type, order_num, text, tool_name, tool_call_id, tool_args_json, tool_result_json, is_error, duration_ms, tokens_input, tokens_output, cost, created_at FROM parts WHERE message_id = ? ORDER BY order_num ASC`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parts := []Part{}
	for rows.Next() {
		p, err := scanPartRow(rows)
		if err != nil {
			return nil, err
		}
		parts = append(parts, *p)
	}
	return parts, rows.Err()
}

func scanSession(row *sql.Row) (*Session, error) {
	return scanSessionRow(row)
}

type rowScanner interface{ Scan(dest ...any) error }

func scanSessionRow(row rowScanner) (*Session, error) {
	var s Session
	var parentID, snapshotHash sql.NullString
	var archivedAt sql.NullInt64
	err := row.Scan(&s.ID, &s.ProjectID, &s.Title, &s.Model, &s.Agent, &parentID, &snapshotHash, &s.TokenInput, &s.TokenOutput, &s.Cost, &s.CreatedAt, &s.UpdatedAt, &archivedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		s.ParentID = &parentID.String
	}
	if snapshotHash.Valid {
		s.SnapshotHash = &snapshotHash.String
	}
	if archivedAt.Valid {
		s.ArchivedAt = &archivedAt.Int64
	}
	return &s, nil
}

func scanPartRow(row rowScanner) (*Part, error) {
	var p Part
	var text, toolName, toolCallID, toolArgs, toolResult sql.NullString
	var duration, tokensIn, tokensOut sql.NullInt64
	var cost sql.NullFloat64
	var isError int
	if err := row.Scan(&p.ID, &p.MessageID, &p.Type, &p.OrderNum, &text, &toolName, &toolCallID, &toolArgs, &toolResult, &isError, &duration, &tokensIn, &tokensOut, &cost, &p.CreatedAt); err != nil {
		return nil, err
	}
	if text.Valid {
		p.Text = &text.String
	}
	if toolName.Valid {
		p.ToolName = &toolName.String
	}
	if toolCallID.Valid {
		p.ToolCallID = &toolCallID.String
	}
	if toolArgs.Valid {
		p.ToolArgsJSON = &toolArgs.String
	}
	if toolResult.Valid {
		p.ToolResultJSON = &toolResult.String
	}
	if duration.Valid {
		p.DurationMS = &duration.Int64
	}
	if tokensIn.Valid {
		p.TokensInput = &tokensIn.Int64
	}
	if tokensOut.Valid {
		p.TokensOutput = &tokensOut.Int64
	}
	if cost.Valid {
		p.Cost = &cost.Float64
	}
	p.IsError = isError == 1
	return &p, nil
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
