package collab

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zero-agent/core/internal/bus"
)

type Service struct {
	store             *Store
	bus               *bus.Bus
	chatHistory       map[string][]ChatMessage
	chatMu            sync.RWMutex
	interruptRequests map[string]*InterruptRequest
	irMu              sync.RWMutex
	sessionRequests   map[string]*SessionRequest
	srMu              sync.RWMutex
}

type PromptExecutor func(ctx context.Context, item *PromptQueueItem) error

func NewService(store *Store, eventBus *bus.Bus) *Service {
	return &Service{
		store:             store,
		bus:               eventBus,
		chatHistory:       make(map[string][]ChatMessage),
		interruptRequests: make(map[string]*InterruptRequest),
		sessionRequests:   make(map[string]*SessionRequest),
	}
}

type CreateRoomInput struct {
	ProjectID    string
	HostClientID string
	Name         string
	Config       RoomConfig
}

type RoomConfig struct {
	DefaultRole                    Role
	PromptReviewMode               PromptReviewMode
	AllowMaintainerPromptIntercept bool
	AllowPromptEditBeforeApproval  bool
	RequireHostApprovalDangerTools bool
	AutoRunQueue                   bool
}

type CreateRoomResult struct {
	Room        *Room  `json:"room"`
	InviteToken string `json:"inviteToken"`
}

func (s *Service) CreateRoom(ctx context.Context, input CreateRoomInput) (*CreateRoomResult, error) {
	token, err := generateInviteToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	cfg := input.Config
	if cfg.DefaultRole == "" {
		cfg.DefaultRole = RolePrompter
	}
	if cfg.PromptReviewMode == "" {
		cfg.PromptReviewMode = ReviewOff
	}

	room := &Room{
		ProjectID:                      input.ProjectID,
		HostClientID:                   input.HostClientID,
		Name:                           input.Name,
		InviteTokenHash:                hashToken(token),
		Status:                         RoomActive,
		DefaultRole:                    cfg.DefaultRole,
		PromptReviewMode:               cfg.PromptReviewMode,
		AllowMaintainerPromptIntercept: cfg.AllowMaintainerPromptIntercept,
		AllowPromptEditBeforeApproval:  cfg.AllowPromptEditBeforeApproval,
		RequireHostApprovalDangerTools: cfg.RequireHostApprovalDangerTools,
		AutoRunQueue:                   cfg.AutoRunQueue,
	}

	if err := s.store.CreateRoom(ctx, room); err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}

	host := &Participant{
		RoomID:      room.ID,
		ClientID:    input.HostClientID,
		DisplayName: "Host",
		Role:        RoleHost,
		Status:      ParticipantOnline,
	}
	if err := s.store.AddParticipant(ctx, host); err != nil {
		return nil, fmt.Errorf("add host participant: %w", err)
	}

	s.bus.PublishRoom(EventRoomCreated, room.ID, input.ProjectID, "", room)

	return &CreateRoomResult{Room: room, InviteToken: token}, nil
}

type JoinRoomInput struct {
	RoomID      string
	Token       string
	ClientID    string
	DisplayName string
}

type JoinRoomResult struct {
	Room        *Room        `json:"room"`
	Participant *Participant `json:"participant"`
}

func (s *Service) JoinRoom(ctx context.Context, input JoinRoomInput) (*JoinRoomResult, error) {
	room, err := s.store.GetRoom(ctx, input.RoomID)
	if err != nil {
		return nil, err
	}

	if room.Status != RoomActive {
		return nil, ErrRoomRevoked
	}

	if hashToken(input.Token) != room.InviteTokenHash {
		return nil, ErrUnauthorized
	}

	participant := &Participant{
		RoomID:      room.ID,
		ClientID:    input.ClientID,
		DisplayName: input.DisplayName,
		Role:        room.DefaultRole,
		Status:      ParticipantOnline,
	}

	if err := s.store.AddParticipant(ctx, participant); err != nil {
		if err == ErrParticipantExists {
			existing, getErr := s.store.GetParticipant(ctx, room.ID, input.ClientID)
			if getErr != nil {
				return nil, getErr
			}
			return &JoinRoomResult{Room: room, Participant: existing}, nil
		}
		return nil, fmt.Errorf("add participant: %w", err)
	}

	s.bus.PublishRoom(EventParticipantJoined, room.ID, room.ProjectID, "", participant)

	joinMsg := &ChatMessage{
		ID:        uuid.New().String(),
		RoomID:    room.ID,
		FromID:    "system",
		Nickname:  "system",
		Role:      "guest",
		Text:      fmt.Sprintf("👋 %s joined the session", participant.DisplayName),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.chatMu.Lock()
	history := s.chatHistory[room.ID]
	history = append(history, *joinMsg)
	if len(history) > 100 {
		history = history[1:]
	}
	s.chatHistory[room.ID] = history
	s.chatMu.Unlock()

	s.bus.PublishRoom(EventChatMessage, room.ID, room.ProjectID, "", joinMsg)

	return &JoinRoomResult{Room: room, Participant: participant}, nil
}

func (s *Service) LeaveRoom(ctx context.Context, roomID, clientID string) error {
	room, err := s.store.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}

	participant, err := s.store.GetParticipant(ctx, roomID, clientID)
	if err != nil {
		return err
	}

	_, err = s.store.db.ExecContext(ctx, `UPDATE collab_participants SET status = 'offline' WHERE room_id = ? AND client_id = ?`, roomID, clientID)
	if err != nil {
		return err
	}

	s.bus.PublishRoom(EventParticipantLeft, room.ID, room.ProjectID, "", participant)
	return nil
}

func (s *Service) RevokeRoom(ctx context.Context, roomID, actorClientID string) error {
	room, err := s.store.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}

	if room.HostClientID != actorClientID {
		actor, err := s.store.GetParticipant(ctx, roomID, actorClientID)
		if err != nil {
			return ErrUnauthorized
		}
		if !HasCapability(actor.Role, CapRevokeRoom) {
			return ErrUnauthorized
		}
	}

	if err := s.store.RevokeRoom(ctx, roomID); err != nil {
		return err
	}

	s.bus.PublishRoom(EventRoomRevoked, room.ID, room.ProjectID, "", room)
	return nil
}

type SubmitPromptInput struct {
	RoomID         string
	SessionID      string
	ActorClientID  string
	Content        string
	IdempotencyKey string
}

type SubmitPromptResult struct {
	QueueItem *PromptQueueItem `json:"queueItem"`
	Queued    bool             `json:"queued"`
	Review    bool             `json:"review"`
}

func (s *Service) SubmitPrompt(ctx context.Context, input SubmitPromptInput) (*SubmitPromptResult, error) {
	if input.IdempotencyKey != "" {
		entry, err := s.store.CheckIdempotency(ctx, input.IdempotencyKey, "prompt")
		if err != nil {
			return nil, err
		}
		if entry != nil {
			var result SubmitPromptResult
			json.Unmarshal([]byte(entry.ResultJSON), &result)
			return &result, nil
		}
	}

	var actorRole Role = RoleHost

	if input.RoomID != "" {
		room, err := s.store.GetRoom(ctx, input.RoomID)
		if err != nil {
			return nil, err
		}
		if room.Status != RoomActive {
			return nil, ErrRoomRevoked
		}

		participant, err := s.store.GetParticipant(ctx, input.RoomID, input.ActorClientID)
		if err != nil {
			return nil, ErrUnauthorized
		}
		actorRole = participant.Role

		if !HasCapability(actorRole, CapSubmitPrompt) {
			return nil, ErrUnauthorized
		}

		needsReview := RequiresReview(room.PromptReviewMode, actorRole)

		status := PromptQueued
		if needsReview {
			status = PromptPendingReview
		}

		roomID := input.RoomID
		item := &PromptQueueItem{
			RoomID:         &roomID,
			SessionID:      input.SessionID,
			ActorClientID:  input.ActorClientID,
			Content:        input.Content,
			Status:         status,
			RequiresReview: needsReview,
		}

		if err := s.store.EnqueuePrompt(ctx, item); err != nil {
			return nil, fmt.Errorf("enqueue: %w", err)
		}

		result := &SubmitPromptResult{
			QueueItem: item,
			Queued:    !needsReview,
			Review:    needsReview,
		}

		if needsReview {
			s.bus.PublishRoom(EventPromptReviewRequired, room.ID, room.ProjectID, input.SessionID, item)
		} else {
			s.bus.PublishRoom(EventPromptQueued, room.ID, room.ProjectID, input.SessionID, item)
		}

		if input.IdempotencyKey != "" {
			resultJSON, _ := json.Marshal(result)
			s.store.SetIdempotency(ctx, input.IdempotencyKey, "prompt", string(resultJSON))
		}

		return result, nil
	}

	item := &PromptQueueItem{
		SessionID:     input.SessionID,
		ActorClientID: input.ActorClientID,
		Content:       input.Content,
		Status:        PromptQueued,
	}
	if err := s.store.EnqueuePrompt(ctx, item); err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}

	result := &SubmitPromptResult{QueueItem: item, Queued: true}
	s.bus.Publish(EventPromptQueued, "", input.SessionID, item)

	if input.IdempotencyKey != "" {
		resultJSON, _ := json.Marshal(result)
		s.store.SetIdempotency(ctx, input.IdempotencyKey, "prompt", string(resultJSON))
	}

	return result, nil
}

type ReviewPromptInput struct {
	RoomID           string
	QueueItemID      string
	ReviewerClientID string
	Action           ReviewAction
	EditedContent    string
	Note             string
}

type ReviewAction string

const (
	ReviewActionApprove ReviewAction = "approve"
	ReviewActionReject  ReviewAction = "reject"
	ReviewActionEdit    ReviewAction = "edit"
)

func (s *Service) ReviewPrompt(ctx context.Context, input ReviewPromptInput) (*PromptQueueItem, error) {
	room, err := s.store.GetRoom(ctx, input.RoomID)
	if err != nil {
		return nil, err
	}

	participant, err := s.store.GetParticipant(ctx, input.RoomID, input.ReviewerClientID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	if !CanReview(participant.Role, room) {
		return nil, ErrUnauthorized
	}

	switch input.Action {
	case ReviewActionApprove:
		if err := s.store.ApprovePrompt(ctx, input.QueueItemID, input.ReviewerClientID, input.Note); err != nil {
			return nil, err
		}
		s.bus.PublishRoom(EventPromptReviewApproved, room.ID, room.ProjectID, "", map[string]string{
			"queueItemId": input.QueueItemID,
			"reviewedBy":  input.ReviewerClientID,
		})

	case ReviewActionReject:
		if err := s.store.RejectPrompt(ctx, input.QueueItemID, input.ReviewerClientID, input.Note); err != nil {
			return nil, err
		}
		s.bus.PublishRoom(EventPromptReviewRejected, room.ID, room.ProjectID, "", map[string]string{
			"queueItemId": input.QueueItemID,
			"reviewedBy":  input.ReviewerClientID,
			"reason":      input.Note,
		})

	case ReviewActionEdit:
		if !room.AllowPromptEditBeforeApproval {
			return nil, ErrUnauthorized
		}
		if err := s.store.EditAndApprovePrompt(ctx, input.QueueItemID, input.ReviewerClientID, input.EditedContent, input.Note); err != nil {
			return nil, err
		}
		s.bus.PublishRoom(EventPromptReviewEdited, room.ID, room.ProjectID, "", map[string]string{
			"queueItemId": input.QueueItemID,
			"reviewedBy":  input.ReviewerClientID,
		})
		s.bus.PublishRoom(EventPromptReviewApproved, room.ID, room.ProjectID, "", map[string]string{
			"queueItemId": input.QueueItemID,
			"reviewedBy":  input.ReviewerClientID,
		})

	default:
		return nil, fmt.Errorf("unknown review action: %s", input.Action)
	}

	return s.store.GetQueueItem(ctx, input.QueueItemID)
}

func (s *Service) CancelPrompt(ctx context.Context, roomID, queueItemID, actorClientID string) error {
	if roomID != "" {
		participant, err := s.store.GetParticipant(ctx, roomID, actorClientID)
		if err != nil {
			return ErrUnauthorized
		}
		if !HasCapability(participant.Role, CapCancelPrompt) {
			return ErrUnauthorized
		}
	}

	if err := s.store.CancelPrompt(ctx, queueItemID); err != nil {
		return err
	}

	s.bus.PublishRoom(EventPromptCancelled, roomID, "", "", map[string]string{
		"queueItemId": queueItemID,
		"cancelledBy": actorClientID,
	})
	return nil
}

func (s *Service) ProcessNextQueuedPrompt(ctx context.Context, sessionID string, exec PromptExecutor) (*PromptQueueItem, error) {
	item, err := s.store.NextQueuedPrompt(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	lock, err := s.store.AcquireSessionLock(ctx, sessionID, item.ActorClientID)
	if err != nil {
		return nil, err
	}

	projectID := ""
	roomID := ""
	if item.RoomID != nil {
		roomID = *item.RoomID
		if room, roomErr := s.store.GetRoom(ctx, roomID); roomErr == nil {
			projectID = room.ProjectID
		}
	}

	s.bus.PublishRoom(EventSessionLocked, roomID, projectID, sessionID, lock)

	if err := s.store.MarkPromptRunning(ctx, item.ID); err != nil {
		s.store.ReleaseSessionLock(ctx, sessionID)
		s.bus.PublishRoom(EventSessionUnlocked, roomID, projectID, sessionID, lock)
		return nil, err
	}
	item, _ = s.store.GetQueueItem(ctx, item.ID)
	s.bus.PublishRoom(EventPromptStarted, roomID, projectID, sessionID, item)

	runErr := exec(ctx, item)
	if runErr != nil {
		_ = s.store.MarkPromptFailed(ctx, item.ID)
		failed, _ := s.store.GetQueueItem(ctx, item.ID)
		s.bus.PublishRoom(EventPromptFailed, roomID, projectID, sessionID, failed)
		_ = s.store.ReleaseSessionLock(ctx, sessionID)
		s.bus.PublishRoom(EventSessionUnlocked, roomID, projectID, sessionID, lock)
		return failed, runErr
	}

	if err := s.store.MarkPromptCompleted(ctx, item.ID); err != nil {
		_ = s.store.ReleaseSessionLock(ctx, sessionID)
		s.bus.PublishRoom(EventSessionUnlocked, roomID, projectID, sessionID, lock)
		return nil, err
	}

	completed, _ := s.store.GetQueueItem(ctx, item.ID)
	s.bus.PublishRoom(EventPromptCompleted, roomID, projectID, sessionID, completed)
	_ = s.store.ReleaseSessionLock(ctx, sessionID)
	s.bus.PublishRoom(EventSessionUnlocked, roomID, projectID, sessionID, lock)
	return completed, nil
}

func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type SendChatInput struct {
	RoomID        string
	ActorClientID string
	Text          string
}

func (s *Service) SendChatMessage(ctx context.Context, input SendChatInput) (*ChatMessage, error) {
	trimmed := strings.TrimSpace(input.Text)
	if trimmed == "" || len(trimmed) > 500 {
		return nil, fmt.Errorf("chat message must be 1-500 characters")
	}

	room, err := s.store.GetRoom(ctx, input.RoomID)
	if err != nil {
		return nil, err
	}
	if room.Status != RoomActive {
		return nil, ErrRoomRevoked
	}

	participant, err := s.store.GetParticipant(ctx, input.RoomID, input.ActorClientID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	msg := &ChatMessage{
		ID:        uuid.New().String(),
		RoomID:    input.RoomID,
		FromID:    input.ActorClientID,
		Nickname:  participant.DisplayName,
		Role:      string(participant.Role),
		Text:      trimmed,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.chatMu.Lock()
	history := s.chatHistory[input.RoomID]
	history = append(history, *msg)
	if len(history) > 100 {
		history = history[1:]
	}
	s.chatHistory[input.RoomID] = history
	s.chatMu.Unlock()

	s.bus.PublishRoom(EventChatMessage, room.ID, room.ProjectID, "", msg)

	return msg, nil
}

func (s *Service) GetChatHistory(ctx context.Context, roomID string) ([]ChatMessage, error) {
	_, err := s.store.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	s.chatMu.RLock()
	history := s.chatHistory[roomID]
	s.chatMu.RUnlock()

	if history == nil {
		return []ChatMessage{}, nil
	}

	result := make([]ChatMessage, len(history))
	copy(result, history)
	return result, nil
}

type InterruptPromptInput struct {
	RoomID        string
	SessionID     string
	ActorClientID string
	CancelRun     func(sessionID string) bool
}

type InterruptPromptResult struct {
	Cancelled         bool              `json:"cancelled"`
	Pending           bool              `json:"pending,omitempty"`
	RequestID         string            `json:"requestId,omitempty"`
	InterruptedActor  string            `json:"interruptedActor,omitempty"`
	InterruptedNick   string            `json:"interruptedNickname,omitempty"`
}

func (s *Service) InterruptPrompt(ctx context.Context, input InterruptPromptInput) (*InterruptPromptResult, error) {
	room, err := s.store.GetRoom(ctx, input.RoomID)
	if err != nil {
		return nil, err
	}

	actor, err := s.store.GetParticipant(ctx, input.RoomID, input.ActorClientID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	var ownerClientID string
	row := s.store.db.QueryRowContext(ctx,
		`SELECT actor_client_id FROM session_run_locks WHERE session_id = ?`,
		input.SessionID)
	_ = row.Scan(&ownerClientID)

	isHost := input.ActorClientID == room.HostClientID
	isOwnSession := ownerClientID == "" || ownerClientID == input.ActorClientID

	if isHost || isOwnSession {
		return s.executeInterrupt(ctx, room, actor, ownerClientID, input)
	}

	owner, _ := s.store.GetParticipant(ctx, input.RoomID, ownerClientID)
	ownerNick := ownerClientID
	if owner != nil {
		ownerNick = owner.DisplayName
	}

	req := &InterruptRequest{
		ID:            uuid.New().String(),
		RoomID:        input.RoomID,
		SessionID:     input.SessionID,
		RequesterID:   input.ActorClientID,
		RequesterNick: actor.DisplayName,
		OwnerID:       ownerClientID,
		OwnerNick:     ownerNick,
		Status:        "pending",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	s.irMu.Lock()
	s.interruptRequests[req.ID] = req
	s.irMu.Unlock()

	s.bus.PublishRoom(EventInterruptRequested, room.ID, room.ProjectID, input.SessionID, req)

	return &InterruptPromptResult{
		Pending:   true,
		RequestID: req.ID,
	}, nil
}

func (s *Service) executeInterrupt(
	ctx context.Context,
	room *Room,
	actor *Participant,
	ownerClientID string,
	input InterruptPromptInput,
) (*InterruptPromptResult, error) {
	result := &InterruptPromptResult{}

	if ownerClientID != "" && ownerClientID != input.ActorClientID {
		if interrupted, getErr := s.store.GetParticipant(ctx, input.RoomID, ownerClientID); getErr == nil {
			result.InterruptedActor = ownerClientID
			result.InterruptedNick = interrupted.DisplayName
		}
	}

	if input.CancelRun != nil {
		result.Cancelled = input.CancelRun(input.SessionID)
	}

	_, _ = s.store.db.ExecContext(ctx,
		`UPDATE prompt_queue SET status = 'cancelled' WHERE session_id = ? AND status IN ('running', 'queued', 'pending_review')`,
		input.SessionID)

	_ = s.store.ReleaseSessionLock(ctx, input.SessionID)

	s.bus.PublishRoom(EventPromptInterrupted, room.ID, room.ProjectID, input.SessionID, map[string]string{
		"sessionId":           input.SessionID,
		"interruptedBy":       input.ActorClientID,
		"interruptedByNick":   actor.DisplayName,
		"interruptedActor":    result.InterruptedActor,
		"interruptedNickname": result.InterruptedNick,
	})

	s.bus.Publish("session.status", "", input.SessionID, map[string]string{
		"status": "cancelled",
	})

	return result, nil
}

type ResolveInterruptInput struct {
	RoomID        string
	RequestID     string
	ActorClientID string
	Approve       bool
	CancelRun     func(sessionID string) bool
}

func (s *Service) ResolveInterrupt(ctx context.Context, input ResolveInterruptInput) (*InterruptPromptResult, error) {
	s.irMu.Lock()
	req, exists := s.interruptRequests[input.RequestID]
	if exists {
		delete(s.interruptRequests, input.RequestID)
	}
	s.irMu.Unlock()

	if !exists || req == nil {
		return nil, fmt.Errorf("interrupt request not found")
	}

	room, err := s.store.GetRoom(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}

	isOwner := input.ActorClientID == req.OwnerID
	isHost := input.ActorClientID == room.HostClientID
	if !isOwner && !isHost {
		return nil, ErrUnauthorized
	}

	if !input.Approve {
		req.Status = "rejected"
		s.bus.PublishRoom(EventInterruptRejected, room.ID, room.ProjectID, req.SessionID, req)
		return &InterruptPromptResult{Cancelled: false}, nil
	}

	req.Status = "approved"
	s.bus.PublishRoom(EventInterruptApproved, room.ID, room.ProjectID, req.SessionID, req)

	requester, _ := s.store.GetParticipant(ctx, req.RoomID, req.RequesterID)
	if requester == nil {
		return nil, ErrUnauthorized
	}

	return s.executeInterrupt(ctx, room, requester, req.OwnerID, InterruptPromptInput{
		RoomID:        req.RoomID,
		SessionID:     req.SessionID,
		ActorClientID: req.RequesterID,
		CancelRun:     input.CancelRun,
	})
}

func (s *Service) RequestSession(ctx context.Context, roomID, actorClientID string) (*SessionRequest, error) {
	room, err := s.store.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	actor, err := s.store.GetParticipant(ctx, roomID, actorClientID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	if actorClientID == room.HostClientID {
		return nil, fmt.Errorf("host does not need to request sessions")
	}

	req := &SessionRequest{
		ID:            uuid.New().String(),
		RoomID:        roomID,
		RequesterID:   actorClientID,
		RequesterNick: actor.DisplayName,
		Status:        "pending",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	s.srMu.Lock()
	s.sessionRequests[req.ID] = req
	s.srMu.Unlock()

	s.bus.PublishRoom(EventSessionRequested, room.ID, room.ProjectID, "", req)

	return req, nil
}

type ResolveSessionRequestInput struct {
	RoomID        string
	RequestID     string
	ActorClientID string
	Approve       bool
	CreateSession func(ctx context.Context, projectID, title, model, agent string) (string, error)
}

type ResolveSessionRequestResult struct {
	SessionID string `json:"sessionId,omitempty"`
	Approved  bool   `json:"approved"`
}

func (s *Service) ResolveSessionRequest(ctx context.Context, input ResolveSessionRequestInput) (*ResolveSessionRequestResult, error) {
	s.srMu.Lock()
	req, exists := s.sessionRequests[input.RequestID]
	if exists {
		delete(s.sessionRequests, input.RequestID)
	}
	s.srMu.Unlock()

	if !exists || req == nil {
		return nil, fmt.Errorf("session request not found")
	}

	room, err := s.store.GetRoom(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}

	if input.ActorClientID != room.HostClientID {
		return nil, ErrUnauthorized
	}

	if !input.Approve {
		req.Status = "rejected"
		s.bus.PublishRoom(EventSessionRejected, room.ID, room.ProjectID, "", req)
		return &ResolveSessionRequestResult{Approved: false}, nil
	}

	req.Status = "approved"

	var sessionID string
	if input.CreateSession != nil {
		title := fmt.Sprintf("%s's session", req.RequesterNick)
		sid, err := input.CreateSession(ctx, room.ProjectID, title, "9router/cb/claude-opus-4.7-1m", "build")
		if err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
		sessionID = sid
	}

	s.bus.PublishRoom(EventSessionApproved, room.ID, room.ProjectID, sessionID, map[string]any{
		"request":   req,
		"sessionId": sessionID,
	})

	return &ResolveSessionRequestResult{Approved: true, SessionID: sessionID}, nil
}
