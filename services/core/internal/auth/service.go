package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zero-agent/core/internal/storage"
)

// AuthStore is the storage surface Service needs. Both the local SQLite
// storage.DB and the new Supabase Postgres adapter implement it. Keeping this
// narrow lets us swap backends without touching handlers or tests.
type AuthStore interface {
	UpsertUser(ctx context.Context, input storage.UpsertUserInput) (*storage.User, error)
	CreateAuthSession(ctx context.Context, userID string, ttl time.Duration) (*storage.AuthSession, error)
	GetAuthSession(ctx context.Context, id string) (*storage.AuthSession, *storage.User, error)
	DeleteAuthSession(ctx context.Context, id string) error
	PurgeExpiredAuthSessions(ctx context.Context) (int64, error)
}

// Service ties OAuth config, cookie secret, dev allowlist, and storage into
// one object the HTTP handlers can call. Construct with NewService at server
// startup.
type Service struct {
	oauth      OAuthConfig
	secret     []byte
	devEmails  []string
	enabled    bool
	store      AuthStore
	httpClient *http.Client

	mu       sync.Mutex
	pending  map[string]pendingFlow
	maxStore int
}

// pendingFlow holds the per-request state we issue when the user clicks "Sign
// in with Google". Keyed by the random `state` we send to Google.
type pendingFlow struct {
	verifier  string
	createdAt time.Time
}

// Config holds everything NewService needs. Pass either DB (legacy SQLite) or
// Store (any AuthStore implementation, e.g. Supabase Postgres). Store wins
// when both are set.
type Config struct {
	OAuth      OAuthConfig
	Secret     []byte
	DevEmails  []string
	Enabled    bool
	DB         *storage.DB
	Store      AuthStore
	HTTPClient *http.Client
}

// NewService constructs an auth Service. When Enabled is false the middleware
// behaves as a no-op and the OAuth handlers still work for testing.
func NewService(cfg Config) (*Service, error) {
	store := cfg.Store
	if store == nil {
		if cfg.DB == nil {
			return nil, errors.New("auth: Store or DB required")
		}
		store = cfg.DB
	}
	if cfg.Enabled {
		if err := cfg.OAuth.Validate(); err != nil {
			return nil, err
		}
		if len(cfg.Secret) < 16 {
			return nil, errors.New("SESSION_SECRET must be at least 16 bytes")
		}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{
		oauth:      cfg.OAuth,
		secret:     append([]byte(nil), cfg.Secret...),
		devEmails:  lowercaseAll(cfg.DevEmails),
		enabled:    cfg.Enabled,
		store:      store,
		httpClient: client,
		pending:    make(map[string]pendingFlow),
		maxStore:   64,
	}, nil
}

// Enabled reports whether auth is enforced. Handlers can short-circuit when
// false (treat every request as anonymous local-dev).
func (s *Service) Enabled() bool { return s.enabled }

// OAuth returns the immutable client config; tests use this.
func (s *Service) OAuth() OAuthConfig { return s.oauth }

// DevEmails returns a copy of the allowlist; tests / dev panel use this.
func (s *Service) DevEmails() []string {
	out := make([]string, len(s.devEmails))
	copy(out, s.devEmails)
	return out
}

// IsDev reports whether the email is in the configured dev list.
func (s *Service) IsDev(email string) bool { return IsDev(email, s.devEmails) }

// Secret returns the cookie HMAC secret. Exposed for handler-side cookie ops.
func (s *Service) Secret() []byte { return s.secret }

// rememberFlow stores the PKCE verifier for the given state. Old entries are
// evicted oldest-first when the map reaches maxStore — auth flows are a few
// minutes long, this map is bounded.
func (s *Service) rememberFlow(state, verifier string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) >= s.maxStore {
		var oldestKey string
		var oldestAt time.Time
		for k, v := range s.pending {
			if oldestKey == "" || v.createdAt.Before(oldestAt) {
				oldestKey = k
				oldestAt = v.createdAt
			}
		}
		if oldestKey != "" {
			delete(s.pending, oldestKey)
		}
	}
	s.pending[state] = pendingFlow{verifier: verifier, createdAt: time.Now()}
}

// consumeFlow returns the verifier for the given state and removes it. Returns
// "" if the state is unknown or older than 10 minutes.
func (s *Service) consumeFlow(state string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.pending[state]
	if !ok {
		return ""
	}
	delete(s.pending, state)
	if time.Since(f.createdAt) > 10*time.Minute {
		return ""
	}
	return f.verifier
}

// BeginFlow mints a new state + PKCE pair, stores the verifier, and returns
// the URL the caller should redirect the browser to.
func (s *Service) BeginFlow() (authURL, state string, err error) {
	state, err = RandomState()
	if err != nil {
		return "", "", err
	}
	pkce, err := NewPkce()
	if err != nil {
		return "", "", err
	}
	s.rememberFlow(state, pkce.Verifier)
	return s.oauth.AuthURL(state, pkce), state, nil
}

// CompleteFlow finishes the OAuth dance: verifies state, exchanges code,
// fetches userinfo, upserts the user, creates an auth_sessions row, and returns
// the new session id ready to put in a cookie.
func (s *Service) CompleteFlow(ctx context.Context, state, code string) (sessionID string, user *storage.User, err error) {
	verifier := s.consumeFlow(state)
	if verifier == "" {
		return "", nil, errors.New("invalid or expired state")
	}
	if code == "" {
		return "", nil, errors.New("missing code")
	}
	tok, err := s.oauth.ExchangeCode(ctx, s.httpClient, code, verifier)
	if err != nil {
		return "", nil, err
	}
	info, err := FetchUserInfo(ctx, s.httpClient, tok.AccessToken)
	if err != nil {
		return "", nil, err
	}
	role := RoleFor(info.Email, s.devEmails)
	upserted, err := s.store.UpsertUser(ctx, storage.UpsertUserInput{
		GoogleID:    info.Sub,
		Email:       info.Email,
		DisplayName: info.Name,
		AvatarURL:   info.Picture,
		Role:        role,
	})
	if err != nil {
		return "", nil, err
	}
	sess, err := s.store.CreateAuthSession(ctx, upserted.ID, SessionTTL)
	if err != nil {
		return "", nil, err
	}
	return sess.ID, upserted, nil
}

// LookupSession reads + validates the cookie, and returns the live user.
// Returns nil with no error when the request has no valid session.
func (s *Service) LookupSession(ctx context.Context, r *http.Request) (*storage.User, *storage.AuthSession, error) {
	sid := ReadSessionID(r, s.secret)
	if sid == "" {
		return nil, nil, nil
	}
	sess, user, err := s.store.GetAuthSession(ctx, sid)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return user, sess, nil
}

// SignOut deletes the server-side row for the cookie's session id (if any) and
// returns the id that was cleared.
func (s *Service) SignOut(ctx context.Context, r *http.Request) (string, error) {
	sid := ReadSessionID(r, s.secret)
	if sid == "" {
		return "", nil
	}
	return sid, s.store.DeleteAuthSession(ctx, sid)
}

// PurgeExpired runs the storage purge; intended for a background ticker.
func (s *Service) PurgeExpired(ctx context.Context) {
	_, _ = s.store.PurgeExpiredAuthSessions(ctx)
}

func lowercaseAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
