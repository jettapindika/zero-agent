package collab_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/collab"
	"github.com/zero-agent/core/internal/storage"
)

func setupTestService(t *testing.T) (*collab.Service, *bus.Bus) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Conn().Exec(`INSERT INTO projects (id, path, name, created_at, updated_at) VALUES ('proj1', '/tmp/proj', 'test-project', 1000, 1000)`)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	_, err = db.Conn().Exec(`INSERT INTO sessions (id, project_id, title, model, agent, created_at, updated_at) VALUES ('sess1', 'proj1', 'test', 'anthropic/claude', 'build', 1000, 1000)`)
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}

	eventBus := bus.New()
	store := collab.NewStore(db.Conn())
	svc := collab.NewService(store, eventBus)
	return svc, eventBus
}

func TestServiceCreateRoom(t *testing.T) {
	svc, eventBus := setupTestService(t)
	ctx := context.Background()

	_, ch := eventBus.Subscribe("proj1", "", 10)

	result, err := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Name:         "my-room",
		Config: collab.RoomConfig{
			DefaultRole:      collab.RolePrompter,
			PromptReviewMode: collab.ReviewHostOnly,
			AutoRunQueue:     true,
		},
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if result.Room.ID == "" {
		t.Fatal("room ID empty")
	}
	if result.InviteToken == "" {
		t.Fatal("invite token empty")
	}
	if result.Room.PromptReviewMode != collab.ReviewHostOnly {
		t.Fatalf("expected host_only, got %s", result.Room.PromptReviewMode)
	}

	event := <-ch
	if event.Type != collab.EventRoomCreated {
		t.Fatalf("expected room.created event, got %s", event.Type)
	}
}

func TestServiceJoinRoom(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Name:         "room",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff, AutoRunQueue: true},
	})

	joinResult, err := svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID:      created.Room.ID,
		Token:       created.InviteToken,
		ClientID:    "client_raka",
		DisplayName: "Raka",
	})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if joinResult.Participant.Role != collab.RolePrompter {
		t.Fatalf("expected prompter role, got %s", joinResult.Participant.Role)
	}
}

func TestServiceJoinRoomBadToken(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff},
	})

	_, err := svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID:      created.Room.ID,
		Token:       "wrong-token",
		ClientID:    "client_raka",
		DisplayName: "Raka",
	})
	if err != collab.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestServiceJoinRevokedRoom(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff},
	})

	svc.RevokeRoom(ctx, created.Room.ID, "client_host")

	_, err := svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID:      created.Room.ID,
		Token:       created.InviteToken,
		ClientID:    "client_raka",
		DisplayName: "Raka",
	})
	if err != collab.ErrRoomRevoked {
		t.Fatalf("expected ErrRoomRevoked, got %v", err)
	}
}

func TestServiceRevokeRoomUnauthorized(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	err := svc.RevokeRoom(ctx, created.Room.ID, "client_raka")
	if err != collab.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestServiceSubmitPromptDirectQueue(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff, AutoRunQueue: true},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	result, err := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID:        created.Room.ID,
		SessionID:     "sess1",
		ActorClientID: "client_raka",
		Content:       "Fix the bug",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !result.Queued {
		t.Fatal("expected queued=true with review off")
	}
	if result.QueueItem.Status != collab.PromptQueued {
		t.Fatalf("expected queued status, got %s", result.QueueItem.Status)
	}
}

func TestServiceSubmitPromptRequiresReview(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewHostOnly, AutoRunQueue: true},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	result, err := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID:        created.Room.ID,
		SessionID:     "sess1",
		ActorClientID: "client_raka",
		Content:       "Refactor auth",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !result.Review {
		t.Fatal("expected review=true with host_only mode")
	}
	if result.QueueItem.Status != collab.PromptPendingReview {
		t.Fatalf("expected pending_review, got %s", result.QueueItem.Status)
	}
}

func TestServiceSubmitPromptHostBypassesReview(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewHostOnly, AutoRunQueue: true},
	})

	result, err := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID:        created.Room.ID,
		SessionID:     "sess1",
		ActorClientID: "client_host",
		Content:       "Host prompt",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Review {
		t.Fatal("host should bypass review")
	}
	if result.QueueItem.Status != collab.PromptQueued {
		t.Fatalf("expected queued, got %s", result.QueueItem.Status)
	}
}

func TestServiceSubmitPromptViewerBlocked(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RoleViewer, PromptReviewMode: collab.ReviewOff},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_viewer", DisplayName: "Viewer",
	})

	_, err := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID:        created.Room.ID,
		SessionID:     "sess1",
		ActorClientID: "client_viewer",
		Content:       "should fail",
	})
	if err != collab.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestServiceReviewPromptApprove(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewHostOnly, AutoRunQueue: true},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	submitted, _ := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID: created.Room.ID, SessionID: "sess1",
		ActorClientID: "client_raka", Content: "Fix bug",
	})

	item, err := svc.ReviewPrompt(ctx, collab.ReviewPromptInput{
		RoomID:           created.Room.ID,
		QueueItemID:      submitted.QueueItem.ID,
		ReviewerClientID: "client_host",
		Action:           collab.ReviewActionApprove,
		Note:             "looks good",
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if item.Status != collab.PromptQueued {
		t.Fatalf("expected queued after approve, got %s", item.Status)
	}
}

func TestServiceReviewPromptReject(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewHostOnly},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	submitted, _ := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID: created.Room.ID, SessionID: "sess1",
		ActorClientID: "client_raka", Content: "Delete everything",
	})

	item, err := svc.ReviewPrompt(ctx, collab.ReviewPromptInput{
		RoomID:           created.Room.ID,
		QueueItemID:      submitted.QueueItem.ID,
		ReviewerClientID: "client_host",
		Action:           collab.ReviewActionReject,
		Note:             "too dangerous",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if item.Status != collab.PromptRejected {
		t.Fatalf("expected rejected, got %s", item.Status)
	}
}

func TestServiceReviewPromptEdit(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config: collab.RoomConfig{
			DefaultRole:                   collab.RolePrompter,
			PromptReviewMode:              collab.ReviewHostOnly,
			AllowPromptEditBeforeApproval: true,
		},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	submitted, _ := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID: created.Room.ID, SessionID: "sess1",
		ActorClientID: "client_raka", Content: "Refactor everything",
	})

	item, err := svc.ReviewPrompt(ctx, collab.ReviewPromptInput{
		RoomID:           created.Room.ID,
		QueueItemID:      submitted.QueueItem.ID,
		ReviewerClientID: "client_host",
		Action:           collab.ReviewActionEdit,
		EditedContent:    "Refactor only auth",
		Note:             "scoped down",
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if item.Status != collab.PromptQueued {
		t.Fatalf("expected queued, got %s", item.Status)
	}
	if item.Content != "Refactor only auth" {
		t.Fatalf("expected edited content, got %s", item.Content)
	}
}

func TestServiceReviewPromptUnauthorizedPrompter(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewHostOnly},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})
	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_dimas", DisplayName: "Dimas",
	})

	submitted, _ := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID: created.Room.ID, SessionID: "sess1",
		ActorClientID: "client_raka", Content: "test",
	})

	_, err := svc.ReviewPrompt(ctx, collab.ReviewPromptInput{
		RoomID:           created.Room.ID,
		QueueItemID:      submitted.QueueItem.ID,
		ReviewerClientID: "client_dimas",
		Action:           collab.ReviewActionApprove,
	})
	if err != collab.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for prompter reviewing, got %v", err)
	}
}

func TestServiceIdempotency(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff, AutoRunQueue: true},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	input := collab.SubmitPromptInput{
		RoomID:         created.Room.ID,
		SessionID:      "sess1",
		ActorClientID:  "client_raka",
		Content:        "Fix bug",
		IdempotencyKey: "idem-key-1",
	}

	r1, err := svc.SubmitPrompt(ctx, input)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	r2, err := svc.SubmitPrompt(ctx, input)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	if r1.QueueItem.ID != r2.QueueItem.ID {
		t.Fatalf("idempotency failed: got different IDs %s vs %s", r1.QueueItem.ID, r2.QueueItem.ID)
	}
}

func TestServiceCancelPromptUnauthorized(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff, AutoRunQueue: true},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	submitted, _ := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID: created.Room.ID, SessionID: "sess1",
		ActorClientID: "client_raka", Content: "test",
	})

	err := svc.CancelPrompt(ctx, created.Room.ID, submitted.QueueItem.ID, "client_raka")
	if err != collab.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for prompter cancelling, got %v", err)
	}

	err = svc.CancelPrompt(ctx, created.Room.ID, submitted.QueueItem.ID, "client_host")
	if err != nil {
		t.Fatalf("host cancel: %v", err)
	}
}

func TestServiceProcessNextQueuedPrompt(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateRoom(ctx, collab.CreateRoomInput{
		ProjectID:    "proj1",
		HostClientID: "client_host",
		Config:       collab.RoomConfig{DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff, AutoRunQueue: true},
	})

	svc.JoinRoom(ctx, collab.JoinRoomInput{
		RoomID: created.Room.ID, Token: created.InviteToken,
		ClientID: "client_raka", DisplayName: "Raka",
	})

	submitted, err := svc.SubmitPrompt(ctx, collab.SubmitPromptInput{
		RoomID: created.Room.ID, SessionID: "sess1",
		ActorClientID: "client_raka", Content: "test",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	called := false
	processed, err := svc.ProcessNextQueuedPrompt(ctx, "sess1", func(ctx context.Context, item *collab.PromptQueueItem) error {
		called = true
		if item.ID != submitted.QueueItem.ID {
			t.Fatalf("expected item %s, got %s", submitted.QueueItem.ID, item.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if !called {
		t.Fatal("executor not called")
	}
	if processed.Status != collab.PromptCompleted {
		t.Fatalf("expected completed, got %s", processed.Status)
	}
}
