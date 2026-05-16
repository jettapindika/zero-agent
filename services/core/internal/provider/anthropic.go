package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type AnthropicConfig struct {
	BaseURL string
	APIKey  string
}

type Anthropic struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewAnthropic(cfg AnthropicConfig) *Anthropic {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &Anthropic{baseURL: baseURL, apiKey: cfg.APIKey, client: http.DefaultClient}
}

func (p *Anthropic) Name() string { return "anthropic" }

func (p *Anthropic) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "anthropic/claude-sonnet-4-5", Name: "claude-sonnet-4-5"},
		{ID: "anthropic/claude-opus-4-5", Name: "claude-opus-4-5"},
		{ID: "anthropic/claude-haiku-4-5", Name: "claude-haiku-4-5"},
	}, nil
}

func (p *Anthropic) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	msgs := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{
		"model":      trimProvider(req.Model),
		"max_tokens": 4096,
		"messages":   msgs,
		"stream":     true,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.applyHeaders(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("anthropic stream failed: %s", res.Status)
	}

	ch := make(chan ChatEvent, 16)
	go func() {
		defer close(ch)
		defer res.Body.Close()
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}
			var event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				ch <- ChatEvent{Delta: event.Delta.Text}
			}
			if event.Type == "message_stop" {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- ChatEvent{Err: err}
		}
	}()
	return ch, nil
}

func (p *Anthropic) GenerateText(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	msgs := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{
		"model":      trimProvider(req.Model),
		"max_tokens": 4096,
		"messages":   msgs,
	})
	if err != nil {
		return ChatResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	p.applyHeaders(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("anthropic generate failed: %s", res.Status)
	}

	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	var text strings.Builder
	for _, block := range payload.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return ChatResponse{Text: text.String()}, nil
}

func (p *Anthropic) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}
}
