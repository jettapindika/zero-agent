package permission_test

import (
	"context"
	"testing"
	"time"

	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/permission"
)

func TestRequestAndApprove(t *testing.T) {
	eventBus := bus.New()
	mgr := permission.NewManager(eventBus)

	_, ch := eventBus.Subscribe("", "sess1", 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		pending := mgr.ListPending("sess1")
		if len(pending) == 0 {
			t.Error("expected pending request")
			return
		}
		mgr.Resolve(pending[0].ID, permission.DecisionAllowOnce)
	}()

	decision, err := mgr.RequestPermission(ctx, "sess1", "bash", map[string]any{"command": "ls"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if decision != permission.DecisionAllowOnce {
		t.Fatalf("expected allow_once, got %s", decision)
	}

	seenRequired := false
	seenResolved := false
	timeout := time.After(500 * time.Millisecond)
	for !seenResolved {
		select {
		case event := <-ch:
			if event.Type == "permission.required" {
				seenRequired = true
			}
			if event.Type == "permission.resolved" {
				seenResolved = true
			}
		case <-timeout:
			break
		}
		if seenResolved {
			break
		}
	}
	if !seenRequired {
		t.Fatal("expected permission.required event")
	}
	if !seenResolved {
		t.Fatal("expected permission.resolved event")
	}
}

func TestAlwaysAllowBypassesFutureRequests(t *testing.T) {
	eventBus := bus.New()
	mgr := permission.NewManager(eventBus)

	ctx := context.Background()

	go func() {
		time.Sleep(50 * time.Millisecond)
		pending := mgr.ListPending("sess1")
		if len(pending) > 0 {
			mgr.Resolve(pending[0].ID, permission.DecisionAlwaysAllow)
		}
	}()

	decision, err := mgr.RequestPermission(ctx, "sess1", "bash", map[string]any{"command": "ls"})
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	if decision != permission.DecisionAlwaysAllow {
		t.Fatalf("expected always_allow, got %s", decision)
	}

	decision, err = mgr.RequestPermission(ctx, "sess1", "bash", map[string]any{"command": "pwd"})
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if decision != permission.DecisionAlwaysAllow {
		t.Fatalf("expected always_allow on second call, got %s", decision)
	}
}

func TestDenyPermission(t *testing.T) {
	eventBus := bus.New()
	mgr := permission.NewManager(eventBus)

	go func() {
		time.Sleep(50 * time.Millisecond)
		pending := mgr.ListPending("sess1")
		if len(pending) > 0 {
			mgr.Resolve(pending[0].ID, permission.DecisionDeny)
		}
	}()

	decision, err := mgr.RequestPermission(context.Background(), "sess1", "write", map[string]any{"path": "x"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if decision != permission.DecisionDeny {
		t.Fatalf("expected deny, got %s", decision)
	}
}

func TestContextCancellationReturnsDeny(t *testing.T) {
	eventBus := bus.New()
	mgr := permission.NewManager(eventBus)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	decision, err := mgr.RequestPermission(ctx, "sess1", "bash", nil)
	if err == nil {
		t.Fatal("expected context error")
	}
	if decision != permission.DecisionDeny {
		t.Fatalf("expected deny on timeout, got %s", decision)
	}
}
