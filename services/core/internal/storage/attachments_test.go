package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zero-agent/core/internal/storage"
)

func newSessionForTest(t *testing.T, db *storage.DB) string {
	t.Helper()
	ctx := context.Background()
	proj, err := db.GetOrCreateProject(ctx, "/tmp/test-attachments", "test")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	s, err := db.CreateSession(ctx, storage.CreateSessionInput{
		ProjectID: proj.ID,
		Title:     "t",
		Model:     "m",
		Agent:     "build",
	})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	return s.ID
}

func TestAttachmentCRUD(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	sessionID := newSessionForTest(t, db)

	extracted := "page text"
	a, err := db.CreateAttachment(ctx, storage.CreateAttachmentInput{
		SessionID:   sessionID,
		OrigName:    "report.pdf",
		MIMEType:    "application/pdf",
		SizeBytes:   12345,
		StoragePath: "/tmp/x/report.pdf",
		Extracted:   &extracted,
		IsChunked:   false,
		ChunkCount:  1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected ID")
	}

	got, err := db.GetAttachment(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OrigName != "report.pdf" || got.SizeBytes != 12345 {
		t.Errorf("roundtrip failed: %+v", got)
	}
	if got.Extracted == nil || *got.Extracted != "page text" {
		t.Errorf("extracted lost: %+v", got.Extracted)
	}

	list, err := db.ListAttachments(ctx, sessionID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	if err := db.SoftDeleteAttachment(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	listAfter, _ := db.ListAttachments(ctx, sessionID)
	if len(listAfter) != 0 {
		t.Errorf("list after delete = %d, want 0", len(listAfter))
	}

	gotDel, err := db.GetAttachment(ctx, a.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if gotDel.DeletedAt == nil {
		t.Error("expected DeletedAt populated after soft delete")
	}

	err = db.SoftDeleteAttachment(ctx, a.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("second delete should return ErrNotFound, got %v", err)
	}
}

func TestAttachmentChunkedFlag(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	sessionID := newSessionForTest(t, db)

	a, err := db.CreateAttachment(ctx, storage.CreateAttachmentInput{
		SessionID:   sessionID,
		OrigName:    "big.txt",
		MIMEType:    "text/plain",
		SizeBytes:   500_000,
		StoragePath: "/tmp/x/big.txt",
		IsChunked:   true,
		ChunkCount:  16,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetAttachment(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.IsChunked || got.ChunkCount != 16 {
		t.Errorf("chunk flags lost: chunked=%v count=%d", got.IsChunked, got.ChunkCount)
	}
}

func TestGetAttachmentNotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := db.GetAttachment(context.Background(), "does-not-exist")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
