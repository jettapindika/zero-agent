package sdk

import (
	"context"
	"net/http"
	"net/url"
)

type Room struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	PromptReviewMode string `json:"promptReviewMode"`
}

type CreateRoomInput struct {
	ProjectID        string `json:"projectId"`
	Name             string `json:"name"`
	DefaultRole      string `json:"defaultRole"`
	PromptReviewMode string `json:"promptReviewMode"`
	AutoRunQueue     bool   `json:"autoRunQueue"`
}

type CreateRoomResult struct {
	Room        Room   `json:"room"`
	InviteToken string `json:"inviteToken"`
}

type JoinRoomResult struct {
	Room        Room        `json:"room"`
	Participant Participant `json:"participant"`
}

type Participant struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type QueueItem struct {
	ID            string `json:"id"`
	ActorClientID string `json:"actorClientId"`
	Content       string `json:"content"`
	Status        string `json:"status"`
	Position      int    `json:"position"`
}

func (c *Client) CreateCollabRoom(ctx context.Context, input CreateRoomInput) (CreateRoomResult, error) {
	var result CreateRoomResult
	err := c.doJSON(ctx, http.MethodPost, "/collab/rooms", input, &result)
	return result, err
}

func (c *Client) JoinCollabRoom(ctx context.Context, roomID, token, displayName string) (JoinRoomResult, error) {
	var result JoinRoomResult
	input := map[string]string{"token": token, "displayName": displayName}
	err := c.doJSON(ctx, http.MethodPost, "/collab/rooms/"+url.PathEscape(roomID)+"/join", input, &result)
	return result, err
}

func (c *Client) ListParticipants(ctx context.Context, roomID string) ([]Participant, error) {
	var participants []Participant
	err := c.doJSON(ctx, http.MethodGet, "/collab/rooms/"+url.PathEscape(roomID)+"/participants", nil, &participants)
	return participants, err
}

func (c *Client) ListPromptQueue(ctx context.Context, roomID, sessionID string) ([]QueueItem, error) {
	var items []QueueItem
	path := "/collab/rooms/" + url.PathEscape(roomID) + "/queue?sessionId=" + url.QueryEscape(sessionID)
	err := c.doJSON(ctx, http.MethodGet, path, nil, &items)
	return items, err
}
