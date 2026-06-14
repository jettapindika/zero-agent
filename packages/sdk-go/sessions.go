package sdk

import (
	"context"
	"net/http"
	"net/url"
)

type Project struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
}

type Session struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Model string `json:"model"`
	Agent string `json:"agent"`
}

type Message struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Type string  `json:"type"`
	Text *string `json:"text"`
}

type CreateSessionInput struct {
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Model     string `json:"model"`
	Agent     string `json:"agent"`
}

type SendMessageInput struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

func (c *Client) EnsureProject(ctx context.Context, path, name string) (Project, error) {
	var project Project
	err := c.doJSON(ctx, http.MethodPost, "/projects/ensure", map[string]string{"path": path, "name": name}, &project)
	return project, err
}

func (c *Client) ListSessions(ctx context.Context, projectID string) ([]Session, error) {
	var sessions []Session
	err := c.doJSON(ctx, http.MethodGet, "/sessions?projectId="+url.QueryEscape(projectID), nil, &sessions)
	return sessions, err
}

func (c *Client) CreateSession(ctx context.Context, input CreateSessionInput) (Session, error) {
	var session Session
	err := c.doJSON(ctx, http.MethodPost, "/sessions", input, &session)
	return session, err
}

func (c *Client) GetSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	var messages []Message
	err := c.doJSON(ctx, http.MethodGet, "/sessions/"+url.PathEscape(sessionID)+"/messages", nil, &messages)
	return messages, err
}

func (c *Client) SendMessage(ctx context.Context, sessionID string, input SendMessageInput) error {
	return c.doJSON(ctx, http.MethodPost, "/sessions/"+url.PathEscape(sessionID)+"/messages", input, nil)
}

func (c *Client) RunSession(ctx context.Context, sessionID string) error {
	return c.doJSON(ctx, http.MethodPost, "/sessions/"+url.PathEscape(sessionID)+"/run", nil, nil)
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	// Hits the provider's OpenAI-compatible /models endpoint. Whatever URL
	// the user configured for ZERO_ROUTER_BASE_URL is what gets queried.
	err := c.doJSON(ctx, http.MethodGet, "/models", nil, &result)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}
