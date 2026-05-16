package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	ProjectID string          `json:"projectId,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	RoomID    string          `json:"roomId,omitempty"`
	ActorID   string          `json:"actorId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt int64           `json:"createdAt"`
}

func (c *Client) SubscribeSession(ctx context.Context, sessionID string) (<-chan Event, context.CancelFunc, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	path := c.BaseURL + "/events?sessionId=" + url.QueryEscape(sessionID)
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, path, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if c.ClientID != "" {
		req.Header.Set("X-Zero-Client-ID", c.ClientID)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		cancel()
		return nil, nil, &APIError{StatusCode: resp.StatusCode}
	}

	events := make(chan Event, 16)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event Event
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				continue
			}
			select {
			case events <- event:
			case <-streamCtx.Done():
				return
			}
		}
	}()

	return events, cancel, nil
}
