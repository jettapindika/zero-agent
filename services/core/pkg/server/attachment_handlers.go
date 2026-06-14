package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/internal/upload"
)

const maxUploadBytes = upload.MaxRequestBytes

func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id required")
		return
	}
	if _, err := s.db.GetSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadBytes))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("multipart parse failed: %v", err))
		return
	}
	form := r.MultipartForm
	if form == nil || len(form.File) == 0 {
		writeError(w, http.StatusBadRequest, "no files in multipart payload")
		return
	}

	sources := make([]upload.SourceFile, 0)
	for _, headers := range form.File {
		for _, h := range headers {
			f, err := h.Open()
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("open part %s: %v", h.Filename, err))
				return
			}
			sources = append(sources, upload.SourceFile{
				OrigName: h.Filename,
				Reader:   f,
				Size:     h.Size,
			})
		}
	}

	results, err := s.uploads.Accept(r.Context(), sessionID, sources)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.injectAttachmentSystemMessage(r.Context(), sessionID, results); err != nil {
		_ = err
	}

	out := make([]storage.Attachment, 0, len(results))
	for _, res := range results {
		out = append(out, res.Attachment)
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id required")
		return
	}
	atts, err := s.db.ListAttachments(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, atts)
}

func (s *Server) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	fileID := chi.URLParam(r, "fileId")
	if sessionID == "" || fileID == "" {
		writeError(w, http.StatusBadRequest, "session id and file id required")
		return
	}
	att, err := s.db.GetAttachment(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "attachment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if att.SessionID != sessionID {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if att.DeletedAt != nil {
		writeError(w, http.StatusGone, "attachment deleted")
		return
	}
	f, err := os.Open(att.StoragePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("open file: %v", err))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", att.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", att.SizeBytes))
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", strings.ReplaceAll(att.OrigName, `"`, `\"`)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	fileID := chi.URLParam(r, "fileId")
	if sessionID == "" || fileID == "" {
		writeError(w, http.StatusBadRequest, "session id and file id required")
		return
	}
	att, err := s.db.GetAttachment(r.Context(), fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "attachment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if att.SessionID != sessionID {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err := s.db.SoftDeleteAttachment(r.Context(), fileID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.uploadStore.Remove(att.StoragePath); err != nil {
		_ = err
	}
	if s.bus != nil {
		s.bus.Publish("attachment.deleted", "", sessionID, map[string]any{
			"id":        fileID,
			"sessionId": sessionID,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) injectAttachmentSystemMessage(ctx context.Context, sessionID string, results []upload.Result) error {
	if len(results) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("User attached file(s) to this session. Use the `attach_read` tool with the matching id to view content.\n\n")
	for _, res := range results {
		a := res.Attachment
		chunkInfo := ""
		if a.IsChunked {
			chunkInfo = fmt.Sprintf(" (chunked: %d, request via {\"id\":%q,\"chunk\":N})", a.ChunkCount, a.ID)
		}
		fmt.Fprintf(&b, "- id=%s · %s · %s · %d bytes%s\n",
			a.ID, a.OrigName, a.MIMEType, a.SizeBytes, chunkInfo)
		if res.Warning != "" {
			fmt.Fprintf(&b, "  warning: %s\n", res.Warning)
		}
	}

	msg, err := s.db.CreateMessage(ctx, sessionID, "system")
	if err != nil {
		return err
	}
	text := b.String()
	if _, err := s.db.CreatePart(ctx, storage.CreatePartInput{
		MessageID: msg.ID,
		Type:      "text",
		OrderNum:  0,
		Text:      &text,
	}); err != nil {
		return err
	}
	if s.bus != nil {
		s.bus.Publish("message.created", "", sessionID, map[string]any{
			"id":        msg.ID,
			"sessionId": sessionID,
			"role":      msg.Role,
		})
	}
	return nil
}


