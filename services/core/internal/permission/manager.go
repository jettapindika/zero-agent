package permission

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/zero-agent/core/internal/bus"
)

type Decision string

const (
	DecisionPending     Decision = "pending"
	DecisionAllowOnce   Decision = "allow_once"
	DecisionAlwaysAllow Decision = "always_allow"
	DecisionDeny        Decision = "deny"
)

type Request struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId"`
	ToolName  string         `json:"toolName"`
	Args      map[string]any `json:"args"`
	Decision  Decision       `json:"decision"`
}

type Manager struct {
	mu       sync.Mutex
	pending  map[string]*pendingRequest
	policies map[string]Decision
	bus      *bus.Bus
}

type pendingRequest struct {
	req  *Request
	done chan Decision
}

func NewManager(eventBus *bus.Bus) *Manager {
	return &Manager{
		pending:  make(map[string]*pendingRequest),
		policies: make(map[string]Decision),
		bus:      eventBus,
	}
}

func (m *Manager) RequestPermission(ctx context.Context, sessionID, toolName string, args map[string]any) (Decision, error) {
	policyKey := fmt.Sprintf("%s:%s", sessionID, toolName)
	m.mu.Lock()
	if policy, ok := m.policies[policyKey]; ok && policy == DecisionAlwaysAllow {
		m.mu.Unlock()
		return DecisionAlwaysAllow, nil
	}

	req := &Request{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		ToolName:  toolName,
		Args:      args,
		Decision:  DecisionPending,
	}
	done := make(chan Decision, 1)
	m.pending[req.ID] = &pendingRequest{req: req, done: done}
	m.mu.Unlock()

	m.bus.Publish("permission.required", "", sessionID, req)

	select {
	case <-ctx.Done():
		m.mu.Lock()
		delete(m.pending, req.ID)
		m.mu.Unlock()
		return DecisionDeny, ctx.Err()
	case decision := <-done:
		return decision, nil
	}
}

func (m *Manager) Resolve(permissionID string, decision Decision) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, ok := m.pending[permissionID]
	if !ok {
		return fmt.Errorf("permission request %s not found", permissionID)
	}

	pending.req.Decision = decision
	if decision == DecisionAlwaysAllow {
		policyKey := fmt.Sprintf("%s:%s", pending.req.SessionID, pending.req.ToolName)
		m.policies[policyKey] = DecisionAlwaysAllow
	}

	pending.done <- decision
	delete(m.pending, permissionID)

	m.bus.Publish("permission.resolved", "", pending.req.SessionID, pending.req)
	return nil
}

func (m *Manager) ListPending(sessionID string) []*Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Request
	for _, p := range m.pending {
		if p.req.SessionID == sessionID {
			result = append(result, p.req)
		}
	}
	return result
}
