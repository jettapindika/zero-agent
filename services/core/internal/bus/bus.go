package bus

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	RoomID    string `json:"roomId,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Payload   any    `json:"payload"`
	CreatedAt int64  `json:"createdAt"`
}

type Subscriber struct {
	ch        chan Event
	roomID    string
	projectID string
	sessionID string
}

type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
}

func New() *Bus {
	return &Bus{
		subscribers: make(map[string]*Subscriber),
	}
}

func (b *Bus) Subscribe(projectID, sessionID string, bufSize int) (string, <-chan Event) {
	return b.SubscribeScoped("", projectID, sessionID, bufSize)
}

func (b *Bus) SubscribeRoom(roomID string, bufSize int) (string, <-chan Event) {
	return b.SubscribeScoped(roomID, "", "", bufSize)
}

func (b *Bus) SubscribeScoped(roomID, projectID, sessionID string, bufSize int) (string, <-chan Event) {
	id := uuid.New().String()
	ch := make(chan Event, bufSize)

	b.mu.Lock()
	b.subscribers[id] = &Subscriber{
		ch:        ch,
		roomID:    roomID,
		projectID: projectID,
		sessionID: sessionID,
	}
	b.mu.Unlock()

	return id, ch
}

func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	if sub, ok := b.subscribers[id]; ok {
		close(sub.ch)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}

func (b *Bus) Publish(eventType, projectID, sessionID string, payload any) {
	b.PublishRoom(eventType, "", projectID, sessionID, payload)
}

func (b *Bus) PublishRoom(eventType, roomID, projectID, sessionID string, payload any) {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		RoomID:    roomID,
		ProjectID: projectID,
		SessionID: sessionID,
		Payload:   payload,
		CreatedAt: time.Now().UnixMilli(),
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if !matches(sub, roomID, projectID, sessionID) {
			continue
		}
		select {
		case sub.ch <- event:
		default:
		}
	}
}

func matches(sub *Subscriber, roomID, projectID, sessionID string) bool {
	if sub.roomID == "" && sub.projectID == "" && sub.sessionID == "" {
		return true
	}
	if sub.roomID != "" && sub.roomID == roomID {
		return true
	}
	if sub.sessionID != "" && sub.sessionID == sessionID {
		return true
	}
	if sub.projectID != "" && sub.projectID == projectID && sub.sessionID == "" {
		return true
	}
	return false
}
