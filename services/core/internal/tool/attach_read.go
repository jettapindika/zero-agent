package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zero-agent/core/internal/storage"
)

type AttachStore interface {
	GetAttachment(ctx context.Context, id string) (*storage.Attachment, error)
	ListAttachments(ctx context.Context, sessionID string) ([]storage.Attachment, error)
}

type attachReadTool struct {
	store AttachStore
}

func AttachRead(store AttachStore) Tool {
	return attachReadTool{store: store}
}

func (attachReadTool) Name() string { return "attach_read" }

func (attachReadTool) Description() string {
	return "Read an uploaded attachment by id. Supports text, PDF, Word (.docx), Excel (.xlsx), and image placeholders. Use the chunk parameter (1-indexed) for chunked attachments. Pass list=true to enumerate attachments in the current session."
}

func (attachReadTool) NeedsPermission() bool { return false }

func (attachReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id":    {"type": "string", "description": "attachment id"},
			"chunk": {"type": "integer", "description": "1-indexed chunk for chunked attachments"},
			"list":  {"type": "boolean", "description": "list all attachments in the current session"}
		}
	}`)
}

func (t attachReadTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		ID    string `json:"id"`
		Chunk int    `json:"chunk"`
		List  bool   `json:"list"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &input); err != nil {
			return Result{}, err
		}
	}

	if input.List || input.ID == "" {
		return t.list(ctx, tc.SessionID)
	}

	att, err := t.store.GetAttachment(ctx, input.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return Result{IsError: true, Output: fmt.Sprintf("attachment %q not found", input.ID)}, nil
		}
		return Result{}, err
	}
	if att.SessionID != tc.SessionID {
		return Result{IsError: true, Output: "attachment does not belong to current session"}, nil
	}
	if att.DeletedAt != nil {
		return Result{IsError: true, Output: "attachment was deleted"}, nil
	}
	if att.Extracted == nil || strings.TrimSpace(*att.Extracted) == "" {
		return Result{
			Title:  att.OrigName,
			Output: fmt.Sprintf("[%s, %s, %d bytes] (no extractable text)", att.OrigName, att.MIMEType, att.SizeBytes),
		}, nil
	}

	body := *att.Extracted
	if !att.IsChunked {
		header := fmt.Sprintf("// Attachment: %s (%s, %d bytes)\n", att.OrigName, att.MIMEType, att.SizeBytes)
		return Result{Title: att.OrigName, Output: header + body}, nil
	}

	chunks := splitIntoChunks(body, att.ChunkCount)
	if input.Chunk <= 0 {
		input.Chunk = 1
	}
	if input.Chunk > len(chunks) {
		return Result{IsError: true, Output: fmt.Sprintf("chunk %d out of range (have %d)", input.Chunk, len(chunks))}, nil
	}
	header := fmt.Sprintf("// Attachment: %s [chunk %d/%d] (%s, %d bytes)\n",
		att.OrigName, input.Chunk, len(chunks), att.MIMEType, att.SizeBytes)
	return Result{Title: att.OrigName, Output: header + chunks[input.Chunk-1]}, nil
}

func (t attachReadTool) list(ctx context.Context, sessionID string) (Result, error) {
	if sessionID == "" {
		return Result{IsError: true, Output: "no session in tool context"}, nil
	}
	atts, err := t.store.ListAttachments(ctx, sessionID)
	if err != nil {
		return Result{}, err
	}
	if len(atts) == 0 {
		return Result{Title: "attachments", Output: "(no attachments)"}, nil
	}
	var b strings.Builder
	for _, a := range atts {
		chunkInfo := ""
		if a.IsChunked {
			chunkInfo = fmt.Sprintf(" [chunked: %d]", a.ChunkCount)
		}
		fmt.Fprintf(&b, "%s\t%s\t%d bytes\t%s%s\n",
			a.ID, a.MIMEType, a.SizeBytes, a.OrigName, chunkInfo)
	}
	return Result{Title: "attachments", Output: b.String()}, nil
}

func splitIntoChunks(text string, n int) []string {
	if n <= 1 {
		return []string{text}
	}
	chunkSize := (len(text) + n - 1) / n
	if chunkSize <= 0 {
		return []string{text}
	}
	out := make([]string, 0, n)
	for i := 0; i < len(text); i += chunkSize {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}
		out = append(out, text[i:end])
	}
	return out
}
