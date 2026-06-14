package upload

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/filehandler"
	"github.com/zero-agent/core/internal/storage"
)

type Result struct {
	Attachment storage.Attachment
	Warning    string
}

type Receiver struct {
	db    *storage.DB
	store *Store
	bus   *bus.Bus
}

func NewReceiver(db *storage.DB, store *Store, eventBus *bus.Bus) *Receiver {
	return &Receiver{db: db, store: store, bus: eventBus}
}

type SourceFile struct {
	OrigName string
	Reader   readerCloser
	Size     int64
}

type readerCloser interface {
	Read(p []byte) (int, error)
	Close() error
}

func (r *Receiver) Accept(ctx context.Context, sessionID string, files []SourceFile) ([]Result, error) {
	if len(files) == 0 {
		return nil, errors.New("no files in upload")
	}
	if _, err := r.db.GetSession(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("session %s: %w", sessionID, err)
	}

	results := make([]Result, 0, len(files))
	for _, src := range files {
		res, err := r.accept(ctx, sessionID, src)
		if err != nil {
			return results, err
		}
		results = append(results, res)

		if r.bus != nil {
			r.bus.Publish("attachment.created", "", sessionID, map[string]any{
				"id":        res.Attachment.ID,
				"sessionId": sessionID,
				"origName":  res.Attachment.OrigName,
				"mimeType":  res.Attachment.MIMEType,
				"sizeBytes": res.Attachment.SizeBytes,
			})
		}
	}
	return results, nil
}

func (r *Receiver) accept(ctx context.Context, sessionID string, src SourceFile) (Result, error) {
	defer src.Reader.Close()

	mime := filehandler.DetectMIME(src.OrigName)
	if !filehandler.IsAllowed(mime) {
		return Result{}, fmt.Errorf("%w: %s (mime=%s)", filehandler.ErrUnsupportedType, src.OrigName, mime)
	}

	attachmentID := strings.ReplaceAll(uuid.New().String(), "-", "")
	ext := filepath.Ext(src.OrigName)

	path, written, err := r.store.Save(sessionID, attachmentID, ext, src.Reader)
	if err != nil {
		return Result{}, err
	}
	if written == 0 {
		_ = r.store.Remove(path)
		return Result{}, filehandler.ErrEmptyFile
	}

	extracted, isChunked, chunkCount, warning := r.extract(path, src.OrigName, mime)

	a, err := r.db.CreateAttachment(ctx, storage.CreateAttachmentInput{
		SessionID:   sessionID,
		OrigName:    src.OrigName,
		MIMEType:    mime,
		SizeBytes:   written,
		StoragePath: path,
		Extracted:   extracted,
		IsChunked:   isChunked,
		ChunkCount:  chunkCount,
	})
	if err != nil {
		_ = r.store.Remove(path)
		return Result{}, fmt.Errorf("persist attachment: %w", err)
	}
	return Result{Attachment: *a, Warning: warning}, nil
}

func (r *Receiver) extract(path, origName, mime string) (*string, bool, int, string) {
	switch {
	case filehandler.IsImage(mime):
		lf, err := filehandler.LoadFile(path, origName)
		if err != nil {
			return nil, false, 0, fmt.Sprintf("image load failed: %v", err)
		}
		text := filehandler.TextPlaceholderForImage(lf)
		return &text, false, 1, ""

	case filehandler.IsTextLike(mime):
		lf, err := filehandler.LoadFile(path, origName)
		if err != nil {
			return nil, false, 0, fmt.Sprintf("text load failed: %v", err)
		}
		if lf.IsChunked {
			joined := strings.Join(lf.Chunks, "")
			return &joined, true, len(lf.Chunks), ""
		}
		return &lf.Content, false, 1, ""

	case mime == filehandler.MIMEPdf:
		pdf, err := filehandler.ReadPDF(path, origName)
		if err != nil {
			return nil, false, 0, fmt.Sprintf("pdf parse failed: %v", err)
		}
		text := pdf.String()
		chunks := filehandler.ChunkText(text, filehandler.ChunkChars)
		if len(chunks) > 1 {
			return &text, true, len(chunks), ""
		}
		return &text, false, 1, ""

	case mime == filehandler.MIMEDocx:
		doc, err := filehandler.ReadDocx(path, origName)
		if err != nil {
			return nil, false, 0, fmt.Sprintf("docx parse failed: %v", err)
		}
		text := doc.String()
		chunks := filehandler.ChunkText(text, filehandler.ChunkChars)
		if len(chunks) > 1 {
			return &text, true, len(chunks), ""
		}
		return &text, false, 1, ""

	case mime == filehandler.MIMEXlsx:
		sheet, err := filehandler.ReadXlsx(path, origName)
		if err != nil {
			return nil, false, 0, fmt.Sprintf("xlsx parse failed: %v", err)
		}
		text := sheet.String()
		chunks := filehandler.ChunkText(text, filehandler.ChunkChars)
		if len(chunks) > 1 {
			return &text, true, len(chunks), ""
		}
		return &text, false, 1, ""
	}
	return nil, false, 0, ""
}
