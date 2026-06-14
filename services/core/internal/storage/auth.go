package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User represents a Google-authenticated identity. Role is "dev" or "user".
type User struct {
	ID          string `json:"id"`
	GoogleID    string `json:"googleId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	Role        string `json:"role"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// AuthSession is a server-side session row keyed by an opaque id stored in the
// signed cookie.
type AuthSession struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	ExpiresAt int64  `json:"expiresAt"`
	CreatedAt int64  `json:"createdAt"`
}

// UpsertUserInput is the payload for UpsertUser; we always overwrite mutable
// profile fields with the freshest values returned by Google.
type UpsertUserInput struct {
	GoogleID    string
	Email       string
	DisplayName string
	AvatarURL   string
	Role        string
}

// UpsertUser inserts a new users row keyed by google_id, or updates the
// mutable profile fields when the row already exists. Returns the resulting
// row.
func (db *DB) UpsertUser(ctx context.Context, input UpsertUserInput) (*User, error) {
	if input.GoogleID == "" {
		return nil, errors.New("google_id required")
	}
	if input.Email == "" {
		return nil, errors.New("email required")
	}
	if input.Role == "" {
		input.Role = "user"
	}
	now := time.Now().UnixMilli()

	// Try insert first; on conflict, update mutable fields.
	id := uuid.New().String()
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO users (id, google_id, email, display_name, avatar_url, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(google_id) DO UPDATE SET
		   email = excluded.email,
		   display_name = excluded.display_name,
		   avatar_url = excluded.avatar_url,
		   role = excluded.role,
		   updated_at = excluded.updated_at`,
		id, input.GoogleID, strings.ToLower(input.Email), input.DisplayName, input.AvatarURL, input.Role, now, now,
	)
	if err != nil {
		return nil, err
	}
	return db.GetUserByGoogleID(ctx, input.GoogleID)
}

// GetUserByID fetches a user row by primary key. Returns ErrNotFound when missing.
func (db *DB) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, google_id, email, display_name, avatar_url, role, created_at, updated_at
		 FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// GetUserByGoogleID fetches a user row by google_id. Returns ErrNotFound when missing.
func (db *DB) GetUserByGoogleID(ctx context.Context, googleID string) (*User, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, google_id, email, display_name, avatar_url, role, created_at, updated_at
		 FROM users WHERE google_id = ?`, googleID)
	return scanUser(row)
}

// GetUserByEmail fetches a user row by lowercased email. Returns ErrNotFound when missing.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, google_id, email, display_name, avatar_url, role, created_at, updated_at
		 FROM users WHERE email = ?`, strings.ToLower(email))
	return scanUser(row)
}

// UpdateUserRole rewrites the role field; useful when DEV_EMAILS changes.
func (db *DB) UpdateUserRole(ctx context.Context, id, role string) error {
	now := time.Now().UnixMilli()
	res, err := db.conn.ExecContext(ctx,
		`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`, role, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.GoogleID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateAuthSession creates a new auth_sessions row that expires after ttl.
// Returns the inserted session.
func (db *DB) CreateAuthSession(ctx context.Context, userID string, ttl time.Duration) (*AuthSession, error) {
	if userID == "" {
		return nil, errors.New("userID required")
	}
	if ttl <= 0 {
		return nil, errors.New("ttl must be positive")
	}
	now := time.Now()
	s := &AuthSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		ExpiresAt: now.Add(ttl).UnixMilli(),
		CreatedAt: now.UnixMilli(),
	}
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO auth_sessions (id, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		s.ID, s.UserID, s.ExpiresAt, s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetAuthSession returns the session row + the owning user. Returns ErrNotFound
// when the session is missing or expired.
func (db *DB) GetAuthSession(ctx context.Context, id string) (*AuthSession, *User, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at, created_at FROM auth_sessions WHERE id = ?`, id)
	var s AuthSession
	if err := row.Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if s.ExpiresAt < time.Now().UnixMilli() {
		// Stale row; lazy-delete and report not found.
		_, _ = db.conn.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id = ?`, id)
		return nil, nil, ErrNotFound
	}
	user, err := db.GetUserByID(ctx, s.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &s, user, nil
}

// DeleteAuthSession removes a session row. Idempotent; missing rows return nil.
func (db *DB) DeleteAuthSession(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id = ?`, id)
	return err
}

// PurgeExpiredAuthSessions deletes any auth_sessions row whose expires_at is in
// the past. Returns the number of rows removed.
func (db *DB) PurgeExpiredAuthSessions(ctx context.Context) (int64, error) {
	now := time.Now().UnixMilli()
	res, err := db.conn.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at < ?`, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
