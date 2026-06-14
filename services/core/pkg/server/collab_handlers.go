package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zero-agent/core/internal/collab"
)

func (s *Server) collabRoutes() chi.Router {
	r := chi.NewRouter()

	r.Post("/rooms", s.handleCreateRoom)
	r.Get("/rooms/{roomId}", s.handleGetRoom)
	r.Post("/rooms/{roomId}/join", s.handleJoinRoom)
	r.Post("/rooms/{roomId}/leave", s.handleLeaveRoom)
	r.Post("/rooms/{roomId}/revoke", s.handleRevokeRoom)
	r.Get("/rooms/{roomId}/participants", s.handleListParticipants)

	r.Post("/rooms/{roomId}/queue", s.handleSubmitPrompt)
	r.Get("/rooms/{roomId}/queue", s.handleListQueue)
	r.Post("/rooms/{roomId}/queue/{queueItemId}/approve", s.handleApprovePrompt)
	r.Post("/rooms/{roomId}/queue/{queueItemId}/reject", s.handleRejectPrompt)
	r.Post("/rooms/{roomId}/queue/{queueItemId}/edit", s.handleEditPrompt)
	r.Post("/rooms/{roomId}/queue/{queueItemId}/cancel", s.handleCancelPrompt)

	r.Get("/rooms/{roomId}/events", s.handleRoomSSE)

	r.Post("/rooms/{roomId}/chat", s.handleSendChat)
	r.Get("/rooms/{roomId}/chat", s.handleGetChatHistory)

	r.Post("/rooms/{roomId}/sessions/{sessionId}/interrupt", s.handleInterruptPrompt)
	r.Post("/rooms/{roomId}/interrupt-requests/{requestId}/resolve", s.handleResolveInterrupt)

	return r
}

type createRoomRequest struct {
	ProjectID                      string `json:"projectId"`
	Name                           string `json:"name"`
	DefaultRole                    string `json:"defaultRole"`
	PromptReviewMode               string `json:"promptReviewMode"`
	AllowMaintainerPromptIntercept *bool  `json:"allowMaintainerPromptIntercept"`
	AllowPromptEditBeforeApproval  *bool  `json:"allowPromptEditBeforeApproval"`
	RequireHostApprovalDangerTools *bool  `json:"requireHostApprovalDangerousTools"`
	AutoRunQueue                   *bool  `json:"autoRunQueue"`
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "projectId required")
		return
	}

	cfg := collab.RoomConfig{
		DefaultRole:                    collab.Role(req.DefaultRole),
		PromptReviewMode:               collab.PromptReviewMode(req.PromptReviewMode),
		AllowMaintainerPromptIntercept: boolDefault(req.AllowMaintainerPromptIntercept, true),
		AllowPromptEditBeforeApproval:  boolDefault(req.AllowPromptEditBeforeApproval, true),
		RequireHostApprovalDangerTools: boolDefault(req.RequireHostApprovalDangerTools, true),
		AutoRunQueue:                   boolDefault(req.AutoRunQueue, true),
	}

	result, err := s.collab.CreateRoom(r.Context(), collab.CreateRoomInput{
		ProjectID:    req.ProjectID,
		HostClientID: clientID,
		Name:         req.Name,
		Config:       cfg,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"room":        result.Room,
		"inviteToken": result.InviteToken,
	})
}

func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	room, err := s.collabStore.GetRoom(r.Context(), roomID)
	if err != nil {
		if err == collab.ErrRoomNotFound {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, room)
}

type joinRoomRequest struct {
	Token       string `json:"token"`
	DisplayName string `json:"displayName"`
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	var req joinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "token and displayName required")
		return
	}

	result, err := s.collab.JoinRoom(r.Context(), collab.JoinRoomInput{
		RoomID:      roomID,
		Token:       req.Token,
		ClientID:    clientID,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		switch err {
		case collab.ErrRoomNotFound:
			writeError(w, http.StatusNotFound, "room not found")
		case collab.ErrRoomRevoked:
			writeError(w, http.StatusGone, "room is revoked")
		case collab.ErrUnauthorized:
			writeError(w, http.StatusUnauthorized, "invalid token")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type resolveInterruptRequest struct {
	Approve bool `json:"approve"`
}

func (s *Server) handleResolveInterrupt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	requestID := chi.URLParam(r, "requestId")

	var req resolveInterruptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	roomID := chi.URLParam(r, "roomId")

	result, err := s.collab.ResolveInterrupt(r.Context(), collab.ResolveInterruptInput{
		RoomID:        roomID,
		RequestID:     requestID,
		ActorClientID: clientID,
		Approve:       req.Approve,
		CancelRun:     s.CancelRun,
	})
	if err != nil {
		switch err {
		case collab.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not authorized")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}
func (s *Server) handleLeaveRoom(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	if err := s.collab.LeaveRoom(r.Context(), roomID, clientID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRevokeRoom(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	if err := s.collab.RevokeRoom(r.Context(), roomID, clientID); err != nil {
		switch err {
		case collab.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not authorized to revoke")
		case collab.ErrRoomNotFound:
			writeError(w, http.StatusNotFound, "room not found")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListParticipants(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	participants, err := s.collabStore.ListParticipants(r.Context(), roomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, participants)
}

type submitPromptRequest struct {
	SessionID      string `json:"sessionId"`
	Content        string `json:"content"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (s *Server) handleSubmitPrompt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	var req submitPromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionID == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "sessionId and content required")
		return
	}

	result, err := s.collab.SubmitPrompt(r.Context(), collab.SubmitPromptInput{
		RoomID:         roomID,
		SessionID:      req.SessionID,
		ActorClientID:  clientID,
		Content:        req.Content,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		switch err {
		case collab.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not authorized to submit prompts")
		case collab.ErrRoomRevoked:
			writeError(w, http.StatusGone, "room is revoked")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleListQueue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId query param required")
		return
	}

	items, err := s.collabStore.ListQueueBySession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []collab.PromptQueueItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

type reviewRequest struct {
	Note string `json:"note"`
}

type editRequest struct {
	Content          string `json:"content"`
	ApproveAfterEdit *bool  `json:"approveAfterEdit"`
	Note             string `json:"note"`
}

func (s *Server) handleApprovePrompt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	queueItemID := chi.URLParam(r, "queueItemId")

	var req reviewRequest
	json.NewDecoder(r.Body).Decode(&req)

	item, err := s.collab.ReviewPrompt(r.Context(), collab.ReviewPromptInput{
		RoomID:           roomID,
		QueueItemID:      queueItemID,
		ReviewerClientID: clientID,
		Action:           collab.ReviewActionApprove,
		Note:             req.Note,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleRejectPrompt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	queueItemID := chi.URLParam(r, "queueItemId")

	var req reviewRequest
	json.NewDecoder(r.Body).Decode(&req)

	item, err := s.collab.ReviewPrompt(r.Context(), collab.ReviewPromptInput{
		RoomID:           roomID,
		QueueItemID:      queueItemID,
		ReviewerClientID: clientID,
		Action:           collab.ReviewActionReject,
		Note:             req.Note,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleEditPrompt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	queueItemID := chi.URLParam(r, "queueItemId")

	var req editRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content required")
		return
	}

	item, err := s.collab.ReviewPrompt(r.Context(), collab.ReviewPromptInput{
		RoomID:           roomID,
		QueueItemID:      queueItemID,
		ReviewerClientID: clientID,
		Action:           collab.ReviewActionEdit,
		EditedContent:    req.Content,
		Note:             req.Note,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCancelPrompt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	queueItemID := chi.URLParam(r, "queueItemId")

	if err := s.collab.CancelPrompt(r.Context(), roomID, queueItemID, clientID); err != nil {
		switch err {
		case collab.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not authorized to cancel")
		case collab.ErrQueueItemNotFound:
			writeError(w, http.StatusNotFound, "queue item not found")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRoomSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	roomID := chi.URLParam(r, "roomId")

	room, err := s.collabStore.GetRoom(r.Context(), roomID)
	if err != nil {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	subID, ch := s.bus.SubscribeRoom(room.ID, 64)
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
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeReviewError(w http.ResponseWriter, err error) {
	switch err {
	case collab.ErrUnauthorized:
		writeError(w, http.StatusForbidden, "not authorized to review")
	case collab.ErrSelfReview:
		writeError(w, http.StatusForbidden, "cannot review own prompt")
	case collab.ErrQueueItemNotFound:
		writeError(w, http.StatusNotFound, "queue item not found")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func boolDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

type sendChatRequest struct {
	Text string `json:"text"`
}

func (s *Server) handleSendChat(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")

	var req sendChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg, err := s.collab.SendChatMessage(r.Context(), collab.SendChatInput{
		RoomID:        roomID,
		ActorClientID: clientID,
		Text:          req.Text,
	})
	if err != nil {
		switch err {
		case collab.ErrRoomNotFound:
			writeError(w, http.StatusNotFound, "room not found")
		case collab.ErrRoomRevoked:
			writeError(w, http.StatusForbidden, "room is revoked")
		case collab.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not a participant")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleGetChatHistory(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")

	history, err := s.collab.GetChatHistory(r.Context(), roomID)
	if err != nil {
		switch err {
		case collab.ErrRoomNotFound:
			writeError(w, http.StatusNotFound, "room not found")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func (s *Server) handleInterruptPrompt(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Zero-Client-ID")
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "missing X-Zero-Client-ID header")
		return
	}

	roomID := chi.URLParam(r, "roomId")
	sessionID := chi.URLParam(r, "sessionId")

	result, err := s.collab.InterruptPrompt(r.Context(), collab.InterruptPromptInput{
		RoomID:        roomID,
		SessionID:     sessionID,
		ActorClientID: clientID,
		CancelRun:     s.CancelRun,
	})
	if err != nil {
		switch err {
		case collab.ErrRoomNotFound:
			writeError(w, http.StatusNotFound, "room not found")
		case collab.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not a participant")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}
