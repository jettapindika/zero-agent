package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zero-agent/core/internal/permission"
	"github.com/zero-agent/core/internal/snapshot"
	"github.com/zero-agent/core/internal/storage"
)

func (s *Server) sessionRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/sessions", s.handleListSessions)
	r.Post("/sessions", s.handleCreateSession)
	r.Get("/sessions/{id}", s.handleGetSession)
	r.Patch("/sessions/{id}", s.handleUpdateSession)
	r.Delete("/sessions/{id}", s.handleDeleteSession)
	r.Post("/sessions/{id}/run", s.handleRunSession)
	r.Post("/sessions/{id}/revert/{hash}", s.handleRevertSession)
	r.Get("/sessions/{id}/permissions", s.handleListPermissions)
	r.Post("/sessions/{id}/permissions/{permissionId}", s.handleResolvePermission)
	r.Post("/sessions/{id}/messages", s.handleCreateMessage)
	r.Get("/sessions/{id}/messages", s.handleListMessages)
	r.Delete("/sessions/{id}/messages/{messageId}", s.handleDeleteMessage)
	return r
}

type createSessionRequest struct {
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Model     string `json:"model"`
	Agent     string `json:"agent"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ProjectID == "" || req.Title == "" {
		writeError(w, http.StatusBadRequest, "projectId and title required")
		return
	}
	if req.Model == "" {
		req.Model = "anthropic/claude-sonnet-4-5"
	}
	if req.Agent == "" {
		req.Agent = "build"
	}

	session, err := s.db.CreateSession(r.Context(), storage.CreateSessionInput{ProjectID: req.ProjectID, Title: req.Title, Model: req.Model, Agent: req.Agent})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.bus.Publish("session.created", session.ProjectID, session.ID, session)
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "projectId query param required")
		return
	}
	sessions, err := s.db.ListSessions(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	session, err := s.db.GetSession(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	session, err := s.db.UpdateSession(r.Context(), chi.URLParam(r, "id"), req.Title)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	s.bus.Publish("session.updated", session.ProjectID, session.ID, session)
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session, _ := s.db.GetSession(r.Context(), id)
	if err := s.db.ArchiveSession(r.Context(), id); err != nil {
		writeStorageError(w, err)
		return
	}
	if session != nil {
		s.bus.Publish("session.deleted", session.ProjectID, session.ID, session)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRunSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if err := s.runner.Run(r.Context(), sessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleRevertSession(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	session, err := s.db.GetSession(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeStorageError(w, err)
		return
	}
	project, err := s.db.GetProjectByPath(r.Context(), "")
	if err != nil || project == nil {
		writeError(w, http.StatusBadRequest, "project not found")
		return
	}
	if err := snapshot.Revert(project.Path, hash); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = session
	w.WriteHeader(http.StatusNoContent)
}

type createMessageRequest struct {
	Role  string                    `json:"role"`
	Text  string                    `json:"text"`
	Parts []storage.CreatePartInput `json:"parts"`
}

func (s *Server) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	var req createMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role == "" {
		writeError(w, http.StatusBadRequest, "role required")
		return
	}

	message, err := s.db.CreateMessage(r.Context(), sessionID, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	parts := []storage.Part{}
	if req.Text != "" {
		part, err := s.db.CreatePart(r.Context(), storage.CreatePartInput{MessageID: message.ID, Type: "text", OrderNum: 0, Text: &req.Text})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		parts = append(parts, *part)
	}
	for i, input := range req.Parts {
		input.MessageID = message.ID
		if input.OrderNum == 0 && len(parts) > 0 {
			input.OrderNum = int64(i + len(parts))
		}
		part, err := s.db.CreatePart(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		parts = append(parts, *part)
	}
	s.bus.Publish("message.created", "", sessionID, map[string]any{"message": message, "parts": parts})
	writeJSON(w, http.StatusCreated, map[string]any{"message": message, "parts": parts})
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	messages, err := s.db.ListMessages(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type messageWithParts struct {
		storage.Message
		Parts []storage.Part `json:"parts"`
	}
	result := []messageWithParts{}
	for _, message := range messages {
		parts, err := s.db.ListParts(r.Context(), message.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, messageWithParts{Message: message, Parts: parts})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "messageId")
	if err := s.db.DeleteMessage(r.Context(), messageID); err != nil {
		writeStorageError(w, err)
		return
	}
	s.bus.Publish("message.updated", "", chi.URLParam(r, "id"), map[string]string{"deletedMessageId": messageID})
	w.WriteHeader(http.StatusNoContent)
}

func writeStorageError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	pending := s.permissions.ListPending(sessionID)
	if pending == nil {
		pending = []*permission.Request{}
	}
	writeJSON(w, http.StatusOK, pending)
}

func (s *Server) handleResolvePermission(w http.ResponseWriter, r *http.Request) {
	permissionID := chi.URLParam(r, "permissionId")
	var req struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	decision := permission.Decision(req.Decision)
	switch decision {
	case permission.DecisionAllowOnce, permission.DecisionAlwaysAllow, permission.DecisionDeny:
	default:
		writeError(w, http.StatusBadRequest, "decision must be allow_once, always_allow, or deny")
		return
	}
	if err := s.permissions.Resolve(permissionID, decision); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
