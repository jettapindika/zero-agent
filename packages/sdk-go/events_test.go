package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSubscribeSessionReadsSSEEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("sessionId"); got != "s1" {
			t.Fatalf("sessionId = %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: part.delta\n")
		fmt.Fprint(w, "data: {\"id\":\"e1\",\"type\":\"part.delta\",\"sessionId\":\"s1\",\"payload\":{\"delta\":\"hi\"},\"createdAt\":1}\n\n")
		fmt.Fprint(w, "event: tool.started\n")
		fmt.Fprint(w, "data: {\"id\":\"e2\",\"type\":\"tool.started\",\"sessionId\":\"s1\",\"payload\":{\"name\":\"read\"},\"createdAt\":2}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, Options{})
	ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second)
	defer cancelCtx()
	events, cancel, err := client.SubscribeSession(ctx, "s1")
	if err != nil {
		t.Fatalf("SubscribeSession error = %v", err)
	}
	defer cancel()

	first := <-events
	if first.Type != "part.delta" || first.SessionID != "s1" || string(first.Payload) != `{"delta":"hi"}` {
		t.Fatalf("first event = %#v payload=%s", first, first.Payload)
	}
	second := <-events
	if second.Type != "tool.started" || string(second.Payload) != `{"name":"read"}` {
		t.Fatalf("second event = %#v payload=%s", second, second.Payload)
	}
}
