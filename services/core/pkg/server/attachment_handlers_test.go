package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/internal/tool"
)

func uploadFile(t *testing.T, baseURL, sessionID, filename, contentType string, body []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="files"; filename="` + filename + `"`}
	if contentType != "" {
		h["Content-Type"] = []string{contentType}
	}
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost,
		baseURL+"/sessions/"+sessionID+"/files", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	return resp
}

func TestAttachmentUploadListAndDelete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := []byte("# hello\nthis is a markdown file\n")
	resp := uploadFile(t, ts.URL, "sess1", "notes.md", "text/markdown", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status = %d, body = %s", resp.StatusCode, b)
	}

	var created []storage.Attachment
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("created len = %d, want 1", len(created))
	}
	att := created[0]
	if att.OrigName != "notes.md" {
		t.Errorf("orig name = %q", att.OrigName)
	}
	if att.MIMEType != "text/markdown" {
		t.Errorf("mime = %q", att.MIMEType)
	}
	if att.SizeBytes != int64(len(body)) {
		t.Errorf("size = %d, want %d", att.SizeBytes, len(body))
	}

	listResp, err := http.Get(ts.URL + "/sessions/sess1/files")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", listResp.StatusCode)
	}
	var listed []storage.Attachment
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != att.ID {
		t.Fatalf("list mismatch: %+v", listed)
	}

	dlResp, err := http.Get(ts.URL + "/sessions/sess1/files/" + att.ID)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", dlResp.StatusCode)
	}
	got, _ := io.ReadAll(dlResp.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("downloaded bytes mismatch")
	}
	if cd := dlResp.Header.Get("Content-Disposition"); !strings.Contains(cd, "notes.md") {
		t.Errorf("content-disposition missing filename: %q", cd)
	}

	delReq, _ := http.NewRequest(http.MethodDelete,
		ts.URL+"/sessions/sess1/files/"+att.ID, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", delResp.StatusCode)
	}

	listResp2, _ := http.Get(ts.URL + "/sessions/sess1/files")
	defer listResp2.Body.Close()
	var listed2 []storage.Attachment
	_ = json.NewDecoder(listResp2.Body).Decode(&listed2)
	if len(listed2) != 0 {
		t.Errorf("after delete, list len = %d, want 0", len(listed2))
	}

	dl2, _ := http.Get(ts.URL + "/sessions/sess1/files/" + att.ID)
	dl2.Body.Close()
	if dl2.StatusCode != http.StatusGone {
		t.Errorf("after delete, download status = %d, want 410", dl2.StatusCode)
	}
}

func TestAttachmentSystemMessageInjected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := []byte("data\n")
	resp := uploadFile(t, ts.URL, "sess1", "x.txt", "text/plain", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}

	mResp, _ := http.Get(ts.URL + "/sessions/sess1/messages")
	defer mResp.Body.Close()

	type msgPayload struct {
		storage.Message
		Parts []storage.Part `json:"parts"`
	}
	var msgs []msgPayload
	if err := json.NewDecoder(mResp.Body).Decode(&msgs); err != nil {
		t.Fatalf("decode messages: %v", err)
	}

	foundSystem := false
	for _, m := range msgs {
		if m.Role != "system" {
			continue
		}
		for _, p := range m.Parts {
			if p.Text != nil && strings.Contains(*p.Text, "attach_read") && strings.Contains(*p.Text, "x.txt") {
				foundSystem = true
				break
			}
		}
	}
	if !foundSystem {
		body, _ := json.Marshal(msgs)
		t.Errorf("expected system message hinting attach_read; messages=%d body=%s", len(msgs), body)
	}
}

func TestAttachmentUploadRejectsUnsupported(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := uploadFile(t, ts.URL, "sess1", "thing.bin", "application/octet-stream", []byte{0, 1, 2})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d, body=%s", resp.StatusCode, b)
	}
}

func TestAttachmentUploadUnknownSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := uploadFile(t, ts.URL, "missing", "x.txt", "text/plain", []byte("x"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAttachReadToolFetchesUploadedFile(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "ar.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	proj, err := db.GetOrCreateProject(ctx, "/tmp/p", "p")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	sess, err := db.CreateSession(ctx, storage.CreateSessionInput{
		ProjectID: proj.ID, Title: "t", Model: "m", Agent: "build",
	})
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	extracted := "// hello\nfunc main(){}\n"
	att, err := db.CreateAttachment(ctx, storage.CreateAttachmentInput{
		SessionID:   sess.ID,
		OrigName:    "main.go",
		MIMEType:    "text/x-go",
		SizeBytes:   int64(len(extracted)),
		StoragePath: "/tmp/dummy.go",
		Extracted:   &extracted,
		ChunkCount:  1,
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	tl := tool.AttachRead(db)
	if tl.Name() != "attach_read" {
		t.Errorf("name = %q", tl.Name())
	}
	if tl.NeedsPermission() {
		t.Error("attach_read should not need permission")
	}

	args, _ := json.Marshal(map[string]any{"id": att.ID})
	res, err := tl.Execute(ctx, args, tool.Context{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.IsError {
		t.Errorf("unexpected error result: %s", res.Output)
	}
	if !strings.Contains(res.Output, "func main()") {
		t.Errorf("expected extracted text in output, got: %q", res.Output)
	}
	if !strings.Contains(res.Output, "main.go") {
		t.Errorf("expected filename in header, got: %q", res.Output)
	}

	listArgs, _ := json.Marshal(map[string]any{"list": true})
	listRes, err := tl.Execute(ctx, listArgs, tool.Context{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("list execute: %v", err)
	}
	if !strings.Contains(listRes.Output, att.ID) {
		t.Errorf("list output missing id: %q", listRes.Output)
	}

	wrongArgs, _ := json.Marshal(map[string]any{"id": att.ID})
	wrongRes, err := tl.Execute(ctx, wrongArgs, tool.Context{SessionID: "different-session"})
	if err != nil {
		t.Fatalf("wrong-session execute: %v", err)
	}
	if !wrongRes.IsError {
		t.Error("wrong-session attach_read should error")
	}

	missingArgs, _ := json.Marshal(map[string]any{"id": "does-not-exist"})
	missingRes, err := tl.Execute(ctx, missingArgs, tool.Context{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("missing execute: %v", err)
	}
	if !missingRes.IsError {
		t.Error("missing id should produce error result")
	}
}
