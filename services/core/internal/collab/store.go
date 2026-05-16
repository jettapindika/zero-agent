package collab

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRoomNotFound       = errors.New("room not found")
	ErrRoomRevoked        = errors.New("room is revoked")
	ErrParticipantExists  = errors.New("participant already exists")
	ErrParticipantNotFound = errors.New("participant not found")
	ErrQueueItemNotFound  = errors.New("queue item not found")
	ErrSelfReview         = errors.New("cannot review own prompt")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrIdempotencyHit     = errors.New("idempotency key already used")
	ErrSessionLocked      = errors.New("session is locked")
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateRoom(ctx context.Context, room *Room) error {
	room.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	room.CreatedAt = now
	room.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO collab_rooms (id, project_id, host_client_id, name, invite_token_hash, status, default_role, prompt_review_mode, allow_maintainer_prompt_intercept, allow_prompt_edit_before_approval, require_host_approval_dangerous_tools, auto_run_queue, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		room.ID, room.ProjectID, room.HostClientID, room.Name, room.InviteTokenHash,
		room.Status, room.DefaultRole, room.PromptReviewMode,
		boolToInt(room.AllowMaintainerPromptIntercept), boolToInt(room.AllowPromptEditBeforeApproval),
		boolToInt(room.RequireHostApprovalDangerTools), boolToInt(room.AutoRunQueue),
		room.CreatedAt, room.UpdatedAt,
	)
	return err
}

func (s *Store) GetRoom(ctx context.Context, roomID string) (*Room, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, host_client_id, name, invite_token_hash, status, default_role, prompt_review_mode, allow_maintainer_prompt_intercept, allow_prompt_edit_before_approval, require_host_approval_dangerous_tools, auto_run_queue, created_at, updated_at, revoked_at
		FROM collab_rooms WHERE id = ?`, roomID)

	var r Room
	var name sql.NullString
	var revokedAt sql.NullInt64
	var maintainerIntercept, editBeforeApproval, hostDanger, autoRun int

	err := row.Scan(&r.ID, &r.ProjectID, &r.HostClientID, &name, &r.InviteTokenHash,
		&r.Status, &r.DefaultRole, &r.PromptReviewMode,
		&maintainerIntercept, &editBeforeApproval, &hostDanger, &autoRun,
		&r.CreatedAt, &r.UpdatedAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, ErrRoomNotFound
	}
	if err != nil {
		return nil, err
	}

	r.Name = name.String
	r.AllowMaintainerPromptIntercept = maintainerIntercept == 1
	r.AllowPromptEditBeforeApproval = editBeforeApproval == 1
	r.RequireHostApprovalDangerTools = hostDanger == 1
	r.AutoRunQueue = autoRun == 1
	if revokedAt.Valid {
		r.RevokedAt = &revokedAt.Int64
	}
	return &r, nil
}

func (s *Store) RevokeRoom(ctx context.Context, roomID string) error {
	now := time.Now().UnixMilli()
	res, err := s.db.ExecContext(ctx, `
		UPDATE collab_rooms SET status = 'revoked', revoked_at = ?, updated_at = ? WHERE id = ? AND status = 'active'`,
		now, now, roomID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrRoomNotFound
	}
	return nil
}

func (s *Store) AddParticipant(ctx context.Context, p *Participant) error {
	p.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	p.JoinedAt = now
	p.LastSeenAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO collab_participants (id, room_id, client_id, display_name, role, status, joined_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.RoomID, p.ClientID, p.DisplayName, p.Role, p.Status, p.JoinedAt, p.LastSeenAt)
	if err != nil && isUniqueViolation(err) {
		return ErrParticipantExists
	}
	return err
}

func (s *Store) GetParticipant(ctx context.Context, roomID, clientID string) (*Participant, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, room_id, client_id, display_name, role, status, joined_at, last_seen_at
		FROM collab_participants WHERE room_id = ? AND client_id = ?`, roomID, clientID)

	var p Participant
	err := row.Scan(&p.ID, &p.RoomID, &p.ClientID, &p.DisplayName, &p.Role, &p.Status, &p.JoinedAt, &p.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, ErrParticipantNotFound
	}
	return &p, err
}

func (s *Store) ListParticipants(ctx context.Context, roomID string) ([]Participant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, room_id, client_id, display_name, role, status, joined_at, last_seen_at
		FROM collab_participants WHERE room_id = ? AND status != 'removed' ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.RoomID, &p.ClientID, &p.DisplayName, &p.Role, &p.Status, &p.JoinedAt, &p.LastSeenAt); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (s *Store) UpdateParticipantRole(ctx context.Context, roomID, clientID string, role Role) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE collab_participants SET role = ? WHERE room_id = ? AND client_id = ?`,
		role, roomID, clientID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrParticipantNotFound
	}
	return nil
}

func (s *Store) EnqueuePrompt(ctx context.Context, item *PromptQueueItem) error {
	item.ID = uuid.New().String()
	item.CreatedAt = time.Now().UnixMilli()

	var maxPos sql.NullInt64
	s.db.QueryRowContext(ctx, `SELECT MAX(position) FROM prompt_queue WHERE session_id = ?`, item.SessionID).Scan(&maxPos)
	item.Position = int(maxPos.Int64) + 1

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO prompt_queue (id, room_id, session_id, actor_client_id, content, original_content, status, requires_review, position, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.RoomID, item.SessionID, item.ActorClientID, item.Content, item.Content,
		item.Status, boolToInt(item.RequiresReview), item.Position, item.CreatedAt)
	return err
}

func (s *Store) GetQueueItem(ctx context.Context, itemID string) (*PromptQueueItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, room_id, session_id, actor_client_id, content, original_content, reviewed_content, status, requires_review, reviewed_by_client_id, reviewed_at, review_note, position, created_at, started_at, completed_at
		FROM prompt_queue WHERE id = ?`, itemID)
	return scanQueueItem(row)
}

func (s *Store) ListQueueBySession(ctx context.Context, sessionID string) ([]PromptQueueItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, room_id, session_id, actor_client_id, content, original_content, reviewed_content, status, requires_review, reviewed_by_client_id, reviewed_at, review_note, position, created_at, started_at, completed_at
		FROM prompt_queue WHERE session_id = ? AND status IN ('queued', 'pending_review', 'submitted', 'running') ORDER BY position ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PromptQueueItem
	for rows.Next() {
		item, err := scanQueueItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) NextQueuedPrompt(ctx context.Context, sessionID string) (*PromptQueueItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, room_id, session_id, actor_client_id, content, original_content, reviewed_content, status, requires_review, reviewed_by_client_id, reviewed_at, review_note, position, created_at, started_at, completed_at
		FROM prompt_queue WHERE session_id = ? AND status = 'queued' ORDER BY position ASC LIMIT 1`, sessionID)
	return scanQueueItem(row)
}

func (s *Store) MarkPromptRunning(ctx context.Context, itemID string) error {
	now := time.Now().UnixMilli()
	res, err := s.db.ExecContext(ctx, `UPDATE prompt_queue SET status = 'running', started_at = ? WHERE id = ? AND status = 'queued'`, now, itemID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrQueueItemNotFound
	}
	return nil
}

func (s *Store) MarkPromptCompleted(ctx context.Context, itemID string) error {
	now := time.Now().UnixMilli()
	res, err := s.db.ExecContext(ctx, `UPDATE prompt_queue SET status = 'completed', completed_at = ? WHERE id = ? AND status = 'running'`, now, itemID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrQueueItemNotFound
	}
	return nil
}

func (s *Store) MarkPromptFailed(ctx context.Context, itemID string) error {
	now := time.Now().UnixMilli()
	res, err := s.db.ExecContext(ctx, `UPDATE prompt_queue SET status = 'failed', completed_at = ? WHERE id = ? AND status = 'running'`, now, itemID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrQueueItemNotFound
	}
	return nil
}

func (s *Store) ApprovePrompt(ctx context.Context, itemID, reviewerClientID, note string) error {
	item, err := s.GetQueueItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.ActorClientID == reviewerClientID {
		return ErrSelfReview
	}
	if item.Status != PromptPendingReview {
		return fmt.Errorf("cannot approve prompt in status %s", item.Status)
	}

	now := time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx, `
		UPDATE prompt_queue SET status = 'queued', reviewed_by_client_id = ?, reviewed_at = ?, review_note = ? WHERE id = ?`,
		reviewerClientID, now, nullStr(note), itemID)
	return err
}

func (s *Store) RejectPrompt(ctx context.Context, itemID, reviewerClientID, reason string) error {
	item, err := s.GetQueueItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.ActorClientID == reviewerClientID {
		return ErrSelfReview
	}
	if item.Status != PromptPendingReview {
		return fmt.Errorf("cannot reject prompt in status %s", item.Status)
	}

	now := time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx, `
		UPDATE prompt_queue SET status = 'rejected', reviewed_by_client_id = ?, reviewed_at = ?, review_note = ? WHERE id = ?`,
		reviewerClientID, now, nullStr(reason), itemID)
	return err
}

func (s *Store) EditAndApprovePrompt(ctx context.Context, itemID, reviewerClientID, newContent, note string) error {
	item, err := s.GetQueueItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.ActorClientID == reviewerClientID {
		return ErrSelfReview
	}
	if item.Status != PromptPendingReview {
		return fmt.Errorf("cannot edit prompt in status %s", item.Status)
	}

	now := time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx, `
		UPDATE prompt_queue SET content = ?, reviewed_content = ?, status = 'queued', reviewed_by_client_id = ?, reviewed_at = ?, review_note = ? WHERE id = ?`,
		newContent, newContent, reviewerClientID, now, nullStr(note), itemID)
	return err
}

func (s *Store) CancelPrompt(ctx context.Context, itemID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE prompt_queue SET status = 'cancelled' WHERE id = ? AND status IN ('queued', 'pending_review', 'submitted')`, itemID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrQueueItemNotFound
	}
	return nil
}

func (s *Store) AcquireSessionLock(ctx context.Context, sessionID, actorClientID string) (*SessionRunLock, error) {
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT session_id FROM session_run_locks WHERE session_id = ?`, sessionID).Scan(&existing)
	if err == nil {
		return nil, ErrSessionLocked
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	lock := &SessionRunLock{
		SessionID:     sessionID,
		RunID:         uuid.New().String(),
		ActorClientID: actorClientID,
		StartedAt:     time.Now().UnixMilli(),
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO session_run_locks (session_id, run_id, actor_client_id, started_at) VALUES (?, ?, ?, ?)`,
		lock.SessionID, lock.RunID, lock.ActorClientID, lock.StartedAt)
	if err != nil {
		return nil, ErrSessionLocked
	}
	return lock, nil
}

func (s *Store) ReleaseSessionLock(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM session_run_locks WHERE session_id = ?`, sessionID)
	return err
}

func (s *Store) CheckIdempotency(ctx context.Context, key, scope string) (*IdempotencyEntry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT key, scope, result_json, created_at FROM idempotency_keys WHERE key = ?`, key)
	var entry IdempotencyEntry
	err := row.Scan(&entry.Key, &entry.Scope, &entry.ResultJSON, &entry.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *Store) SetIdempotency(ctx context.Context, key, scope, resultJSON string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (key, scope, result_json, created_at) VALUES (?, ?, ?, ?)`,
		key, scope, resultJSON, time.Now().UnixMilli())
	return err
}

func scanQueueItem(row *sql.Row) (*PromptQueueItem, error) {
	var item PromptQueueItem
	var roomID, originalContent, reviewedContent, reviewedBy, reviewNote sql.NullString
	var reviewedAt, startedAt, completedAt sql.NullInt64
	var requiresReview int

	err := row.Scan(&item.ID, &roomID, &item.SessionID, &item.ActorClientID, &item.Content,
		&originalContent, &reviewedContent, &item.Status, &requiresReview,
		&reviewedBy, &reviewedAt, &reviewNote,
		&item.Position, &item.CreatedAt, &startedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, ErrQueueItemNotFound
	}
	if err != nil {
		return nil, err
	}

	if roomID.Valid {
		item.RoomID = &roomID.String
	}
	if originalContent.Valid {
		item.OriginalContent = &originalContent.String
	}
	if reviewedContent.Valid {
		item.ReviewedContent = &reviewedContent.String
	}
	if reviewedBy.Valid {
		item.ReviewedByClientID = &reviewedBy.String
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Int64
	}
	if reviewNote.Valid {
		item.ReviewNote = &reviewNote.String
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		item.CompletedAt = &completedAt.Int64
	}
	item.RequiresReview = requiresReview == 1
	return &item, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanQueueItemRow(row scannable) (*PromptQueueItem, error) {
	var item PromptQueueItem
	var roomID, originalContent, reviewedContent, reviewedBy, reviewNote sql.NullString
	var reviewedAt, startedAt, completedAt sql.NullInt64
	var requiresReview int

	err := row.Scan(&item.ID, &roomID, &item.SessionID, &item.ActorClientID, &item.Content,
		&originalContent, &reviewedContent, &item.Status, &requiresReview,
		&reviewedBy, &reviewedAt, &reviewNote,
		&item.Position, &item.CreatedAt, &startedAt, &completedAt)
	if err != nil {
		return nil, err
	}

	if roomID.Valid {
		item.RoomID = &roomID.String
	}
	if originalContent.Valid {
		item.OriginalContent = &originalContent.String
	}
	if reviewedContent.Valid {
		item.ReviewedContent = &reviewedContent.String
	}
	if reviewedBy.Valid {
		item.ReviewedByClientID = &reviewedBy.String
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Int64
	}
	if reviewNote.Valid {
		item.ReviewNote = &reviewNote.String
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		item.CompletedAt = &completedAt.Int64
	}
	item.RequiresReview = requiresReview == 1
	return &item, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func isUniqueViolation(err error) bool {
	return err != nil && (errors.Is(err, sql.ErrNoRows) || containsUniqueConstraint(err))
}

func containsUniqueConstraint(err error) bool {
	return err != nil && (len(err.Error()) > 0 && (contains(err.Error(), "UNIQUE") || contains(err.Error(), "unique")))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
