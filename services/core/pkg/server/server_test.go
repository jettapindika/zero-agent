package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/pkg/server"
)

func setupTestServer(t *testing.T) *httptest.Server {
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
	srv := server.New(db, eventBus)
	return httptest.NewServer(srv)
}

func TestHealthEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("get health: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLocalCORSPreflightForDesktop(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest("OPTIONS", ts.URL+"/projects/ensure", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "http://tauri.localhost")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://tauri.localhost" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("expected allowed headers")
	}
}

func TestCollabFullFlow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createBody, _ := json.Marshal(map[string]any{
		"projectId":        "proj1",
		"name":             "test-room",
		"defaultRole":      "prompter",
		"promptReviewMode": "host_only",
		"autoRunQueue":     true,
	})
	req, _ := http.NewRequest("POST", ts.URL+"/collab/rooms", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_host")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var createResult struct {
		Room        map[string]any `json:"room"`
		InviteToken string         `json:"inviteToken"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)
	resp.Body.Close()

	roomID := createResult.Room["id"].(string)
	token := createResult.InviteToken

	if roomID == "" || token == "" {
		t.Fatal("room ID or token empty")
	}

	joinBody, _ := json.Marshal(map[string]string{
		"token":       token,
		"displayName": "Raka",
	})
	req, _ = http.NewRequest("POST", ts.URL+"/collab/rooms/"+roomID+"/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_raka")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("join room: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("join expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	req, _ = http.NewRequest("GET", ts.URL+"/collab/rooms/"+roomID+"/participants", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	var participants []map[string]any
	json.NewDecoder(resp.Body).Decode(&participants)
	resp.Body.Close()

	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}

	promptBody, _ := json.Marshal(map[string]string{
		"sessionId": "sess1",
		"content":   "Fix the auth bug",
	})
	req, _ = http.NewRequest("POST", ts.URL+"/collab/rooms/"+roomID+"/queue", bytes.NewReader(promptBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_raka")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("submit prompt: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("submit expected 201, got %d", resp.StatusCode)
	}

	var submitResult struct {
		QueueItem map[string]any `json:"QueueItem"`
		Review    bool           `json:"Review"`
	}
	json.NewDecoder(resp.Body).Decode(&submitResult)
	resp.Body.Close()

	if !submitResult.Review {
		t.Fatal("expected review=true for prompter in host_only mode")
	}

	queueItemID := submitResult.QueueItem["id"].(string)

	approveBody, _ := json.Marshal(map[string]string{"note": "approved"})
	req, _ = http.NewRequest("POST", ts.URL+"/collab/rooms/"+roomID+"/queue/"+queueItemID+"/approve", bytes.NewReader(approveBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_host")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("approve expected 200, got %d", resp.StatusCode)
	}

	var approvedItem map[string]any
	json.NewDecoder(resp.Body).Decode(&approvedItem)
	resp.Body.Close()

	if approvedItem["status"] != "queued" {
		t.Fatalf("expected queued after approval, got %s", approvedItem["status"])
	}
}

func TestCollabBadToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createBody, _ := json.Marshal(map[string]any{
		"projectId":        "proj1",
		"promptReviewMode": "off",
	})
	req, _ := http.NewRequest("POST", ts.URL+"/collab/rooms", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_host")
	resp, _ := http.DefaultClient.Do(req)

	var createResult struct {
		Room map[string]any `json:"room"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)
	resp.Body.Close()
	roomID := createResult.Room["id"].(string)

	joinBody, _ := json.Marshal(map[string]string{
		"token":       "wrong-token",
		"displayName": "Hacker",
	})
	req, _ = http.NewRequest("POST", ts.URL+"/collab/rooms/"+roomID+"/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_hacker")

	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for bad token, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCollabViewerCannotPrompt(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createBody, _ := json.Marshal(map[string]any{
		"projectId":        "proj1",
		"defaultRole":      "viewer",
		"promptReviewMode": "off",
	})
	req, _ := http.NewRequest("POST", ts.URL+"/collab/rooms", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_host")
	resp, _ := http.DefaultClient.Do(req)

	var createResult struct {
		Room        map[string]any `json:"room"`
		InviteToken string         `json:"inviteToken"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)
	resp.Body.Close()
	roomID := createResult.Room["id"].(string)

	joinBody, _ := json.Marshal(map[string]string{
		"token":       createResult.InviteToken,
		"displayName": "Viewer",
	})
	req, _ = http.NewRequest("POST", ts.URL+"/collab/rooms/"+roomID+"/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_viewer")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	promptBody, _ := json.Marshal(map[string]string{
		"sessionId": "sess1",
		"content":   "should fail",
	})
	req, _ = http.NewRequest("POST", ts.URL+"/collab/rooms/"+roomID+"/queue", bytes.NewReader(promptBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zero-Client-ID", "client_viewer")

	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for viewer prompting, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSessionAndMessageHTTPFlow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createSessionBody, _ := json.Marshal(map[string]string{
		"projectId": "proj1",
		"title":     "Auth refactor",
		"model":     "anthropic/claude-sonnet-4-5",
		"agent":     "build",
	})
	resp, err := http.Post(ts.URL+"/sessions", "application/json", bytes.NewReader(createSessionBody))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session expected 201, got %d", resp.StatusCode)
	}
	var session map[string]any
	json.NewDecoder(resp.Body).Decode(&session)
	resp.Body.Close()
	sessionID := session["id"].(string)

	messageBody, _ := json.Marshal(map[string]string{
		"role": "user",
		"text": "Hello Zero",
	})
	resp, err = http.Post(ts.URL+"/sessions/"+sessionID+"/messages", "application/json", bytes.NewReader(messageBody))
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create message expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/sessions/" + sessionID + "/messages")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list messages expected 200, got %d", resp.StatusCode)
	}
	var messages []map[string]any
	json.NewDecoder(resp.Body).Decode(&messages)
	resp.Body.Close()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}

func TestPatchSessionUpdatesModel(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := bytes.NewBufferString(`{"model":"openai/gpt-4o-mini"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/sessions/sess1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Model != "openai/gpt-4o-mini" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.Title != "test" {
		t.Fatalf("title should be unchanged, got %q", got.Title)
	}
}

func TestPatchSessionRejectsModelWithoutProviderPrefix(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := bytes.NewBufferString(`{"model":"gpt-5.5"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/sessions/sess1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPatchSessionTitleOnlyStillWorks(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := bytes.NewBufferString(`{"title":"Renamed Session"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/sessions/sess1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got struct {
		Title string `json:"title"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Title != "Renamed Session" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Model != "anthropic/claude" {
		t.Fatalf("model should be unchanged, got %q", got.Model)
	}
}

func TestPatchSessionRejectsEmptyBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := bytes.NewBufferString(`{}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/sessions/sess1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPatchSessionRejectsUnknownAgent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := bytes.NewBufferString(`{"agent":"hacker"}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/sessions/sess1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCancelSessionWhenNothingRunningReturnsFalse(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/sessions/sess1/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("post cancel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got struct {
		Cancelled bool `json:"cancelled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Cancelled {
		t.Fatalf("expected cancelled=false when no run is in flight")
	}
}
