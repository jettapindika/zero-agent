package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupabaseAuthStore implements the auth.AuthStore surface against a Supabase
// Postgres instance. Only the auth-related tables (`users`, `auth_sessions`)
// live here; chat data stays in local SQLite. Wire by setting
// ZERO_SUPABASE_DB_URL in the daemon env.
type SupabaseAuthStore struct {
	pool *pgxpool.Pool
}

// NewSupabaseAuthStore opens a pgx connection pool to the Supabase Postgres
// instance and verifies it with a single ping. Caller must Close().
//
// Connection string format (Supabase shows it in Project Settings → Database):
//
//	postgresql://postgres:<password>@db.<project-ref>.supabase.co:5432/postgres
func NewSupabaseAuthStore(ctx context.Context, dsn string) (*SupabaseAuthStore, error) {
	if dsn == "" {
		return nil, errors.New("ZERO_SUPABASE_DB_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse supabase dsn: %w", err)
	}
	// Pool tuning for the auth path:
	//   - Auth is low-traffic so a small ceiling is fine; Supabase free tier
	//     also enforces low connection caps.
	//   - 30m idle matches pgx's own default (kept explicit for clarity).
	//   - HealthCheckPeriod ensures dead conns (network blip, pooler restart)
	//     are evicted, otherwise the next /auth/me would hang on a stale fd.
	cfg.MaxConns = 5
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute
	// Per-attempt connect timeout. Without this the daemon blocks for the
	// kernel TCP timeout (~75s) when the DSN is wrong or IPv6 is broken.
	cfg.ConnConfig.ConnectTimeout = 8 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("supabase pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("supabase ping: %w", err)
	}
	return &SupabaseAuthStore{pool: pool}, nil
}

// Close releases the connection pool.
func (s *SupabaseAuthStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// UpsertUser mirrors the SQLite version but uses Postgres-flavored upsert and
// `bigint` (millis) for timestamps to stay schema-compatible with our local
// tables. See services/core/supabase/migrations/00001_auth.sql.
func (s *SupabaseAuthStore) UpsertUser(ctx context.Context, input UpsertUserInput) (*User, error) {
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
	id := uuid.New().String()

	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, google_id, email, display_name, avatar_url, role, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (google_id) DO UPDATE SET
		   email = EXCLUDED.email,
		   display_name = EXCLUDED.display_name,
		   avatar_url = EXCLUDED.avatar_url,
		   role = EXCLUDED.role,
		   updated_at = EXCLUDED.updated_at`,
		id, input.GoogleID, strings.ToLower(input.Email), input.DisplayName, input.AvatarURL, input.Role, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("supabase upsert user: %w", err)
	}

	row := s.pool.QueryRow(ctx,
		`SELECT id, google_id, email, display_name, avatar_url, role, created_at, updated_at
		 FROM users WHERE google_id = $1`, input.GoogleID,
	)
	return scanUserPgx(row)
}

// GetUserByID fetches a user row by primary key.
func (s *SupabaseAuthStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, google_id, email, display_name, avatar_url, role, created_at, updated_at
		 FROM users WHERE id = $1`, id)
	return scanUserPgx(row)
}

// CreateAuthSession writes a new server-side session row.
func (s *SupabaseAuthStore) CreateAuthSession(ctx context.Context, userID string, ttl time.Duration) (*AuthSession, error) {
	if userID == "" {
		return nil, errors.New("userID required")
	}
	if ttl <= 0 {
		return nil, errors.New("ttl must be positive")
	}
	now := time.Now()
	sess := &AuthSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		ExpiresAt: now.Add(ttl).UnixMilli(),
		CreatedAt: now.UnixMilli(),
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_sessions (id, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4)`,
		sess.ID, sess.UserID, sess.ExpiresAt, sess.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("supabase create session: %w", err)
	}
	return sess, nil
}

// GetAuthSession returns the session row + owning user. Stale rows are
// lazy-deleted to keep `auth_sessions` tidy without a cron.
func (s *SupabaseAuthStore) GetAuthSession(ctx context.Context, id string) (*AuthSession, *User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, expires_at, created_at FROM auth_sessions WHERE id = $1`, id)
	var sess AuthSession
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if sess.ExpiresAt < time.Now().UnixMilli() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE id = $1`, id)
		return nil, nil, ErrNotFound
	}
	user, err := s.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &sess, user, nil
}

// DeleteAuthSession is idempotent.
func (s *SupabaseAuthStore) DeleteAuthSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE id = $1`, id)
	return err
}

// PurgeExpiredAuthSessions removes any auth_sessions row whose expires_at is
// in the past.
func (s *SupabaseAuthStore) PurgeExpiredAuthSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at < $1`, time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// scanUserPgx is the pgx counterpart of scanUser; returns ErrNotFound for
// pgx.ErrNoRows so callers can branch identically across backends.
func scanUserPgx(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.GoogleID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
