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

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
}

type OpenAI struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAI(cfg OpenAIConfig) *OpenAI {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAI{baseURL: baseURL, apiKey: cfg.APIKey, client: http.DefaultClient}
}

func (p *OpenAI) Name() string {
	return "openai"
}

func (p *OpenAI) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{ID: "openai/gpt-4o-mini", Name: "gpt-4o-mini"}, {ID: "openai/gpt-4o", Name: "gpt-4o"}}, nil
}

func (p *OpenAI) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	body, err := json.Marshal(map[string]any{"model": req.Model, "messages": req.Messages, "stream": true})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
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
		return nil, fmt.Errorf("openai stream failed: %s", res.Status)
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
			if data == "[DONE]" {
				return
			}
			delta, err := parseStreamDelta([]byte(data))
			if err != nil {
				ch <- ChatEvent{Err: err}
				return
			}
			if delta != "" {
				ch <- ChatEvent{Delta: delta}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- ChatEvent{Err: err}
		}
	}()
	return ch, nil
}

func (p *OpenAI) GenerateText(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body, err := json.Marshal(map[string]any{"model": req.Model, "messages": req.Messages})
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
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
		return ChatResponse{}, fmt.Errorf("openai generate failed: %s", res.Status)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, nil
	}
	return ChatResponse{Text: payload.Choices[0].Message.Content}, nil
}

func (p *OpenAI) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

func parseStreamDelta(data []byte) (string, error) {
	var payload struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", nil
	}
	return payload.Choices[0].Delta.Content, nil
}

func trimProvider(model string) string {
	_, after, ok := strings.Cut(model, "/")
	if ok {
		return after
	}
	return model
}
