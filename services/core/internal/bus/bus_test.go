package bus_test

import (
	"testing"
	"time"

	"github.com/zero-agent/core/internal/bus"
)

func TestPublishSubscribe(t *testing.T) {
	b := bus.New()

	_, ch := b.Subscribe("", "", 10)

	b.Publish("session.created", "proj1", "sess1", map[string]string{"title": "test"})

	select {
	case event := <-ch:
		if event.Type != "session.created" {
			t.Fatalf("expected session.created, got %s", event.Type)
		}
		if event.ProjectID != "proj1" {
			t.Fatalf("expected proj1, got %s", event.ProjectID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestProjectScopedSubscription(t *testing.T) {
	b := bus.New()

	_, ch := b.Subscribe("proj1", "", 10)

	b.Publish("session.created", "proj2", "sess1", nil)
	b.Publish("session.created", "proj1", "sess2", nil)

	select {
	case event := <-ch:
		if event.ProjectID != "proj1" {
			t.Fatalf("expected proj1 event, got %s", event.ProjectID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSessionScopedSubscription(t *testing.T) {
	b := bus.New()

	_, ch := b.Subscribe("", "sess1", 10)

	b.Publish("part.delta", "proj1", "sess2", nil)
	b.Publish("part.delta", "proj1", "sess1", nil)

	select {
	case event := <-ch:
		if event.SessionID != "sess1" {
			t.Fatalf("expected sess1 event, got %s", event.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := bus.New()

	id, ch := b.Subscribe("", "", 10)
	b.Unsubscribe(id)

	b.Publish("test", "", "", nil)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
	}
}
