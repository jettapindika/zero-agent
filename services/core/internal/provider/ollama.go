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

type OllamaConfig struct {
	BaseURL string
}

type Ollama struct {
	baseURL string
	client  *http.Client
}

func NewOllama(cfg OllamaConfig) *Ollama {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Ollama{baseURL: baseURL, client: http.DefaultClient}
}

func (p *Ollama) Name() string { return "ollama" }

func (p *Ollama) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	json.NewDecoder(res.Body).Decode(&payload)
	models := make([]ModelInfo, 0, len(payload.Models))
	for _, m := range payload.Models {
		models = append(models, ModelInfo{ID: "ollama/" + m.Name, Name: m.Name})
	}
	return models, nil
}

func (p *Ollama) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	msgs := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{"model": trimProvider(req.Model), "messages": msgs, "stream": true})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("ollama stream failed: %s", res.Status)
	}

	ch := make(chan ChatEvent, 16)
	go func() {
		defer close(ch)
		defer res.Body.Close()
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				continue
			}
			if chunk.Message.Content != "" {
				ch <- ChatEvent{Delta: chunk.Message.Content}
			}
			if chunk.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- ChatEvent{Err: err}
		}
	}()
	return ch, nil
}

func (p *Ollama) GenerateText(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	msgs := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{"model": trimProvider(req.Model), "messages": msgs, "stream": false})
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("ollama generate failed: %s", res.Status)
	}
	var payload struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	json.NewDecoder(res.Body).Decode(&payload)
	return ChatResponse{Text: payload.Message.Content}, nil
}
