package provider

import "context"

type Provider interface {
	Name() string
	ListModels(ctx context.Context) ([]ModelInfo, error)
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
	GenerateText(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatEvent struct {
	Delta string
	Err   error
}

type ChatResponse struct {
	Text string
}
