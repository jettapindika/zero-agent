package collab_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zero-agent/core/internal/collab"
	"github.com/zero-agent/core/internal/storage"
)

func setupTestStore(t *testing.T) *collab.Store {
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

	return collab.NewStore(db.Conn())
}

func TestCreateAndGetRoom(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	room := &collab.Room{
		ProjectID:                      "proj1",
		HostClientID:                   "client_host",
		Name:                           "test-room",
		InviteTokenHash:                "hash123",
		Status:                         collab.RoomActive,
		DefaultRole:                    collab.RolePrompter,
		PromptReviewMode:               collab.ReviewHostOnly,
		AllowMaintainerPromptIntercept: true,
		AllowPromptEditBeforeApproval:  true,
		RequireHostApprovalDangerTools: true,
		AutoRunQueue:                   true,
	}

	if err := store.CreateRoom(ctx, room); err != nil {
		t.Fatalf("create room: %v", err)
	}
	if room.ID == "" {
		t.Fatal("room ID not set")
	}

	got, err := store.GetRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if got.Name != "test-room" {
		t.Fatalf("expected name test-room, got %s", got.Name)
	}
	if got.PromptReviewMode != collab.ReviewHostOnly {
		t.Fatalf("expected host_only review mode, got %s", got.PromptReviewMode)
	}
	if !got.AutoRunQueue {
		t.Fatal("expected auto_run_queue true")
	}
}

func TestRevokeRoom(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	room := &collab.Room{
		ProjectID:       "proj1",
		HostClientID:    "client_host",
		InviteTokenHash: "hash456",
		Status:          collab.RoomActive,
		DefaultRole:     collab.RolePrompter,
		PromptReviewMode: collab.ReviewOff,
	}
	store.CreateRoom(ctx, room)

	if err := store.RevokeRoom(ctx, room.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, _ := store.GetRoom(ctx, room.ID)
	if got.Status != collab.RoomRevoked {
		t.Fatalf("expected revoked, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Fatal("expected revoked_at set")
	}
}

func TestAddAndListParticipants(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	room := &collab.Room{
		ProjectID: "proj1", HostClientID: "client_host",
		InviteTokenHash: "h", Status: collab.RoomActive,
		DefaultRole: collab.RolePrompter, PromptReviewMode: collab.ReviewOff,
	}
	store.CreateRoom(ctx, room)

	p := &collab.Participant{
		RoomID: room.ID, ClientID: "client_raka",
		DisplayName: "Raka", Role: collab.RolePrompter, Status: collab.ParticipantOnline,
	}
	if err := store.AddParticipant(ctx, p); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	err := store.AddParticipant(ctx, &collab.Participant{
		RoomID: room.ID, ClientID: "client_raka",
		DisplayName: "Raka2", Role: collab.RolePrompter, Status: collab.ParticipantOnline,
	})
	if err != collab.ErrParticipantExists {
		t.Fatalf("expected ErrParticipantExists, got %v", err)
	}

	list, err := store.ListParticipants(ctx, room.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(list))
	}
	if list[0].DisplayName != "Raka" {
		t.Fatalf("expected Raka, got %s", list[0].DisplayName)
	}
}

func TestEnqueueAndApprovePrompt(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	item := &collab.PromptQueueItem{
		SessionID:     "sess1",
		ActorClientID: "client_raka",
		Content:       "Refactor auth middleware",
		Status:        collab.PromptPendingReview,
		RequiresReview: true,
	}
	if err := store.EnqueuePrompt(ctx, item); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, err := store.GetQueueItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != collab.PromptPendingReview {
		t.Fatalf("expected pending_review, got %s", got.Status)
	}

	if err := store.ApprovePrompt(ctx, item.ID, "client_host", "looks good"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	got, _ = store.GetQueueItem(ctx, item.ID)
	if got.Status != collab.PromptQueued {
		t.Fatalf("expected queued after approval, got %s", got.Status)
	}
}

func TestRejectPrompt(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	item := &collab.PromptQueueItem{
		SessionID:     "sess1",
		ActorClientID: "client_raka",
		Content:       "Delete everything",
		Status:        collab.PromptPendingReview,
		RequiresReview: true,
	}
	store.EnqueuePrompt(ctx, item)

	if err := store.RejectPrompt(ctx, item.ID, "client_host", "too broad"); err != nil {
		t.Fatalf("reject: %v", err)
	}

	got, _ := store.GetQueueItem(ctx, item.ID)
	if got.Status != collab.PromptRejected {
		t.Fatalf("expected rejected, got %s", got.Status)
	}
}

func TestEditAndApprovePrompt(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	item := &collab.PromptQueueItem{
		SessionID:     "sess1",
		ActorClientID: "client_raka",
		Content:       "Refactor everything",
		Status:        collab.PromptPendingReview,
		RequiresReview: true,
	}
	store.EnqueuePrompt(ctx, item)

	newContent := "Refactor only auth middleware"
	if err := store.EditAndApprovePrompt(ctx, item.ID, "client_host", newContent, "scoped down"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	got, _ := store.GetQueueItem(ctx, item.ID)
	if got.Status != collab.PromptQueued {
		t.Fatalf("expected queued, got %s", got.Status)
	}
	if got.Content != newContent {
		t.Fatalf("expected edited content, got %s", got.Content)
	}
	if got.OriginalContent == nil || *got.OriginalContent != "Refactor everything" {
		t.Fatal("original content not preserved")
	}
	if got.ReviewedContent == nil || *got.ReviewedContent != newContent {
		t.Fatal("reviewed content not set")
	}
}

func TestSelfReviewBlocked(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	item := &collab.PromptQueueItem{
		SessionID:     "sess1",
		ActorClientID: "client_raka",
		Content:       "test",
		Status:        collab.PromptPendingReview,
		RequiresReview: true,
	}
	store.EnqueuePrompt(ctx, item)

	err := store.ApprovePrompt(ctx, item.ID, "client_raka", "")
	if err != collab.ErrSelfReview {
		t.Fatalf("expected ErrSelfReview, got %v", err)
	}

	err = store.RejectPrompt(ctx, item.ID, "client_raka", "")
	if err != collab.ErrSelfReview {
		t.Fatalf("expected ErrSelfReview, got %v", err)
	}

	err = store.EditAndApprovePrompt(ctx, item.ID, "client_raka", "new", "")
	if err != collab.ErrSelfReview {
		t.Fatalf("expected ErrSelfReview, got %v", err)
	}
}

func TestSessionLock(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	lock, err := store.AcquireSessionLock(ctx, "sess1", "client_host")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if lock.RunID == "" {
		t.Fatal("expected run ID")
	}

	_, err = store.AcquireSessionLock(ctx, "sess1", "client_raka")
	if err != collab.ErrSessionLocked {
		t.Fatalf("expected ErrSessionLocked, got %v", err)
	}

	if err := store.ReleaseSessionLock(ctx, "sess1"); err != nil {
		t.Fatalf("release: %v", err)
	}

	_, err = store.AcquireSessionLock(ctx, "sess1", "client_raka")
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestIdempotency(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entry, err := store.CheckIdempotency(ctx, "key1", "prompt")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if entry != nil {
		t.Fatal("expected nil for new key")
	}

	if err := store.SetIdempotency(ctx, "key1", "prompt", `{"id":"q1"}`); err != nil {
		t.Fatalf("set: %v", err)
	}

	entry, err = store.CheckIdempotency(ctx, "key1", "prompt")
	if err != nil {
		t.Fatalf("check after set: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.ResultJSON != `{"id":"q1"}` {
		t.Fatalf("expected result json, got %s", entry.ResultJSON)
	}
}

func TestPromptQueueOrdering(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	for i, content := range []string{"first", "second", "third"} {
		_ = i
		item := &collab.PromptQueueItem{
			SessionID:     "sess1",
			ActorClientID: "client_raka",
			Content:       content,
			Status:        collab.PromptQueued,
		}
		store.EnqueuePrompt(ctx, item)
	}

	items, err := store.ListQueueBySession(ctx, "sess1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Content != "first" || items[1].Content != "second" || items[2].Content != "third" {
		t.Fatalf("wrong order: %s, %s, %s", items[0].Content, items[1].Content, items[2].Content)
	}
	if items[0].Position >= items[1].Position || items[1].Position >= items[2].Position {
		t.Fatal("positions not ascending")
	}
}
