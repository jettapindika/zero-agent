package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zero-agent/core/internal/agent"
	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/collab"
	"github.com/zero-agent/core/internal/permission"
	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/internal/tool"
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
	router      chi.Router
}

func New(db *storage.DB, eventBus *bus.Bus) *Server {
	collabStore := collab.NewStore(db.Conn())
	collabSvc := collab.NewService(collabStore, eventBus)

	routerBaseURL := os.Getenv("ZERO_ROUTER_BASE_URL")
	if routerBaseURL == "" {
		routerBaseURL = "http://127.0.0.1:20128/v1"
	}
	routerAPIKey := os.Getenv("ZERO_ROUTER_API_KEY")
	if routerAPIKey == "" {
		routerAPIKey = "sk_9router"
	}
	aiProvider := provider.NewOpenAI(provider.OpenAIConfig{BaseURL: routerBaseURL, APIKey: routerAPIKey})
	permMgr := permission.NewManager(eventBus)
	toolExecutor := agent.NewToolExecutor(tool.DefaultRegistry(), permMgr, eventBus)

	s := &Server{
		db:          db,
		bus:         eventBus,
		collab:      collabSvc,
		collabStore: collabStore,
		runner:      agent.NewRunnerWithExecutor(db, eventBus, aiProvider, toolExecutor),
		permissions: permMgr,
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

	r.Get("/health", s.handleHealth)
	r.Get("/events", s.handleSSE)
	r.Get("/openapi.json", s.handleOpenAPI)
	r.Post("/projects/ensure", s.handleEnsureProject)
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Zero-Client-ID")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok"}`)
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
	srv := New(db, eventBus)

	slog.Info("starting zero server", "port", cfg.Port, "db", cfg.DBPath)
	return http.ListenAndServe(":"+cfg.Port, srv)
}
