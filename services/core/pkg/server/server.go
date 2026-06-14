package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zero-agent/core/internal/agent"
	"github.com/zero-agent/core/internal/auth"
	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/collab"
	"github.com/zero-agent/core/internal/permission"
	"github.com/zero-agent/core/pkg/identity"
	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/internal/tool"
	"github.com/zero-agent/core/internal/upload"
)

type Config struct {
	Port   string
	DBPath string
}

func DefaultConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8910"
	}
	return Config{
		Port:   port,
		DBPath: storage.DefaultPath(),
	}
}

type Server struct {
	db          *storage.DB
	bus         *bus.Bus
	collab      *collab.Service
	collabStore *collab.Store
	runner      *agent.Runner
	permissions *permission.Manager
	aiProvider  provider.Provider
	auth        *auth.Service
	uploads     *upload.Receiver
	uploadStore *upload.Store
	router      chi.Router

	runsMu sync.Mutex
	runs   map[string]context.CancelFunc // sessionID -> cancel
}

// New constructs a Server without an auth gate. Existing single-user installs
// keep working unchanged. Use NewWithAuth to opt into Google login.
func New(db *storage.DB, eventBus *bus.Bus) *Server {
	return NewWithAuth(db, eventBus, nil)
}

// NewWithAuth constructs a Server with an optional auth Service. When the
// service is nil, behavior is identical to New. When non-nil, /auth/* routes
// are registered and (if Service.Enabled()) every other route is gated.
func NewWithAuth(db *storage.DB, eventBus *bus.Bus, authSvc *auth.Service) *Server {
	collabStore := collab.NewStore(db.Conn())
	collabSvc := collab.NewService(collabStore, eventBus)

	// Provider URL + key are user-supplied. Zero is OpenAI-compatible so any
	// `/v1` endpoint that serves `GET /models` and `POST /chat/completions`
	// will work (OpenAI, OpenRouter, LiteLLM, Ollama, vLLM, llama.cpp, ...).
	// Defaults to public OpenAI; users override via env or `~/.config/zero/.env`.
	routerBaseURL := os.Getenv("ZERO_ROUTER_BASE_URL")
	if routerBaseURL == "" {
		routerBaseURL = "https://api.openai.com/v1"
	}
	routerAPIKey := os.Getenv("ZERO_ROUTER_API_KEY")
	aiProvider := provider.NewOpenAI(provider.OpenAIConfig{BaseURL: routerBaseURL, APIKey: routerAPIKey})
	permMgr := permission.NewManager(eventBus)

	uploadStore := upload.NewStore(upload.DefaultRoot())
	uploadReceiver := upload.NewReceiver(db, uploadStore, eventBus)

	tools := tool.DefaultRegistry()
	tools.Register(tool.AttachRead(db))
	toolExecutor := agent.NewToolExecutor(tools, permMgr, eventBus)

	s := &Server{
		db:          db,
		bus:         eventBus,
		collab:      collabSvc,
		collabStore: collabStore,
		runner:      agent.NewRunnerWithExecutor(db, eventBus, aiProvider, toolExecutor),
		permissions: permMgr,
		aiProvider:  aiProvider,
		auth:        authSvc,
		uploads:     uploadReceiver,
		uploadStore: uploadStore,
		runs:        make(map[string]context.CancelFunc),
	}
	s.router = s.routes()
	return s
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(localCORSMiddleware)
	if s.auth != nil {
		r.Use(s.auth.RequireAuth)
	}

	r.Get("/health", s.handleHealth)
	r.Get("/events", s.handleSSE)
	r.Get("/openapi.json", s.handleOpenAPI)
	r.Get("/providers/models", s.handleListModels)
	r.Get("/identity", s.handleIdentity)
	r.Post("/projects/ensure", s.handleEnsureProject)

	if s.auth != nil {
		r.Get("/auth/google/start", s.handleAuthStart)
		r.Get("/auth/google/callback", s.handleAuthCallback)
		r.Get("/auth/me", s.handleAuthMe)
		r.Post("/auth/logout", s.handleAuthLogout)

		// Dev-only routes — require auth + dev role.
		r.Group(func(dev chi.Router) {
			dev.Use(s.auth.RequireDev)
			dev.Get("/dev/runtime", s.handleDevRuntime)
			dev.Post("/dev/skills/reload", s.handleDevReloadSkills)
		})
	} else {
		// Auth is disabled — register helpful stubs so misconfigured clients
		// see a clear 503 with remediation, not a silent 404. The stubs share
		// one handler that explains how to turn auth on.
		r.Get("/auth/google/start", handleAuthDisabled)
		r.Get("/auth/google/callback", handleAuthDisabled)
		r.Get("/auth/me", handleAuthDisabled)
		r.Post("/auth/logout", handleAuthDisabled)
	}

	r.Mount("/", s.sessionRoutes())
	r.Mount("/collab", s.collabRoutes())

	return r
}

func localCORSMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"http://127.0.0.1:3200": true,
		"http://localhost:3200":  true,
		"http://tauri.localhost":  true,
		"tauri://localhost":       true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Zero-Client-ID, Authorization")
			// Cookies must travel from tauri.localhost to 127.0.0.1:8910 so the
			// auth session cookie reaches the daemon on every fetch.
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// beginRun registers a cancel func for the given session and returns it. If
// another run is already in flight, the previous one is cancelled first so a
// fresh /run replaces it.
func (s *Server) beginRun(parent context.Context, sessionID string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	s.runsMu.Lock()
	if existing, ok := s.runs[sessionID]; ok {
		existing()
	}
	s.runs[sessionID] = cancel
	s.runsMu.Unlock()
	return ctx, cancel
}

func (s *Server) endRun(sessionID string, cancel context.CancelFunc) {
	cancel()
	s.runsMu.Lock()
	if current, ok := s.runs[sessionID]; ok {
		// Only delete if the stored cancel matches; otherwise a newer run owns it.
		// Pointer equality of two CancelFuncs is unreliable across closures, so
		// always delete here — endRun is only called when our run is finishing.
		_ = current
		delete(s.runs, sessionID)
	}
	s.runsMu.Unlock()
}

// CancelRun aborts an in-flight run for the given session. Returns true if a
// run was cancelled, false if nothing was running.
func (s *Server) CancelRun(sessionID string) bool {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	cancel, ok := s.runs[sessionID]
	if !ok {
		return false
	}
	cancel()
	delete(s.runs, sessionID)
	return true
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	if s.aiProvider == nil {
		writeJSON(w, http.StatusOK, []provider.ModelInfo{})
		return
	}
	models, err := s.aiProvider.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if models == nil {
		models = []provider.ModelInfo{}
	}
	writeJSON(w, http.StatusOK, models)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok"}`)
}

// handleIdentity returns the local client identity stored at ~/.zero/client.json.
// The desktop app uses this to populate the X-Zero-Client-ID header on collab
// endpoints. Auto-generated on first read so a fresh install works without
// running `zero setup` first.
func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	id, err := identity.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, id)
}

func (s *Server) handleEnsureProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	if req.Name == "" {
		req.Name = req.Path
	}
	project, err := s.db.GetOrCreateProject(r.Context(), req.Path, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	projectID := r.URL.Query().Get("projectId")
	sessionID := r.URL.Query().Get("sessionId")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	subID, ch := s.bus.Subscribe(projectID, sessionID, 64)
	defer s.bus.Unsubscribe(subID)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("marshal sse event", "error", err)
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func Start(cfg Config) error {
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	eventBus := bus.New()

	// Auth is opt-in via ZERO_AUTH_ENABLED. When unset/false the daemon
	// behaves exactly as it has historically (single-user, no login screen).
	authSvc, err := buildAuthService(db)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	srv := NewWithAuth(db, eventBus, authSvc)

	// Background ticker purges expired auth_sessions rows every hour. Cheap
	// SQL DELETE; safe to run alongside live traffic.
	if authSvc != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					authSvc.PurgeExpired(context.Background())
				}
			}
		}()
	}

	slog.Info("starting zero server", "port", cfg.Port, "db", cfg.DBPath, "auth", authSvc != nil)
	return http.ListenAndServe(":"+cfg.Port, srv)
}

// Bundled Google OAuth client values, injected at build time via
// `-ldflags "-X github.com/zero-agent/core/pkg/server.defaultGoogleClientID=...
//           -X github.com/zero-agent/core/pkg/server.defaultGoogleClientSecret=..."`.
// See the `build` target in Makefile and the `.build-secrets` file (gitignored)
// for the actual values shipped with release binaries.
//
// Why ldflags and not const literals: Google's "Desktop application" OAuth
// client_secret is, per their docs, not treated as a secret — but GitHub's
// secret-scanning push protection still flags the literal in source. Keeping
// the values out of git also keeps them out of forks' history and makes
// rotation a build-config change instead of a source-code change.
//
// When unset (e.g., a contributor builds from source without the secrets
// file), env vars GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET are still honored,
// so the BYO-Google-client path keeps working.
var (
	defaultGoogleClientID     = ""
	defaultGoogleClientSecret = ""
)

// buildAuthService reads env vars and constructs an auth.Service. Returns nil
// (no error) when ZERO_AUTH_ENABLED is unset or false.
func buildAuthService(db *storage.DB) (*auth.Service, error) {
	enabled := os.Getenv("ZERO_AUTH_ENABLED") == "true" || os.Getenv("ZERO_AUTH_ENABLED") == "1"
	if !enabled {
		return nil, nil
	}

	callback := os.Getenv("GOOGLE_CALLBACK_URL")
	if callback == "" {
		callback = "http://127.0.0.1:8910/auth/google/callback"
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required when ZERO_AUTH_ENABLED=true")
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = defaultGoogleClientID
	}
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = defaultGoogleClientSecret
	}

	cfg := auth.Config{
		OAuth: auth.OAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			CallbackURL:  callback,
		},
		Secret:    []byte(secret),
		DevEmails: auth.DevEmails(),
		Enabled:   true,
		DB:        db,
	}

	// When ZERO_SUPABASE_DB_URL is set, route auth-table writes to Supabase
	// Postgres instead of local SQLite. Chat data still lives in SQLite.
	// On Supabase failure (DNS, IPv6, wrong password) we DO NOT crash the
	// daemon — we log loudly and fall back to local SQLite so chat keeps
	// working. The user can fix the DSN and restart.
	if dsn := os.Getenv("ZERO_SUPABASE_DB_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		store, err := storage.NewSupabaseAuthStore(ctx, dsn)
		if err != nil {
			slog.Error("auth backend = supabase unreachable; falling back to local sqlite",
				"host", supabaseHost(dsn),
				"error", err.Error(),
				"hint", "use the Supavisor session pooler DSN if your network is IPv4-only")
		} else {
			slog.Info("auth backend = supabase postgres", "host", supabaseHost(dsn))
			cfg.Store = store
		}
	}
	if cfg.Store == nil {
		slog.Info("auth backend = local sqlite")
	}

	return auth.NewService(cfg)
}

// supabaseHost extracts the Supabase host from a DSN for logging without
// leaking the password. Returns "<unknown>" when parsing fails.
func supabaseHost(dsn string) string {
	// Look for "@host:" pattern; cheaper than url.Parse and fine for logs.
	if i := strings.Index(dsn, "@"); i >= 0 {
		rest := dsn[i+1:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return "<unknown>"
}
