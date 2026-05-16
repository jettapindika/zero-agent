package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureProjectPostsPathAndName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/ensure" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["path"] != "/repo" || body["name"] != "repo" {
			t.Fatalf("unexpected body %#v", body)
		}
		_ = json.NewEncoder(w).Encode(Project{ID: "p1", Path: "/repo", Name: "repo"})
	}))
	defer server.Close()

	client := NewClient(server.URL, Options{})
	project, err := client.EnsureProject(context.Background(), "/repo", "repo")
	if err != nil {
		t.Fatalf("EnsureProject returned error: %v", err)
	}
	if project.ID != "p1" {
		t.Fatalf("project ID = %q, want p1", project.ID)
	}
}

func TestCreateRoomSendsClientHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Zero-Client-ID"); got != "client-1" {
			t.Fatalf("client header = %q", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/collab/rooms" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CreateRoomResult{InviteToken: "tok", Room: Room{ID: "room-1"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, Options{ClientID: "client-1"})
	result, err := client.CreateCollabRoom(context.Background(), CreateRoomInput{ProjectID: "p1", Name: "repo"})
	if err != nil {
		t.Fatalf("CreateCollabRoom returned error: %v", err)
	}
	if result.Room.ID != "room-1" || result.InviteToken != "tok" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestListSessionsUsesProjectQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sessions" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("projectId"); got != "p1" {
			t.Fatalf("projectId = %q, want p1", got)
		}
		_ = json.NewEncoder(w).Encode([]Session{{ID: "s1", Title: "terminal"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, Options{})
	sessions, err := client.ListSessions(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Fatalf("unexpected sessions %#v", sessions)
	}
}

func TestAPIErrorIncludesStatusAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, Options{})
	err := client.Health(context.Background())
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Body == "" {
		t.Fatalf("unexpected APIError %#v", apiErr)
	}
}
