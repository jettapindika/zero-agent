package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Options struct {
	ClientID string
	HTTP     *http.Client
}

type Client struct {
	BaseURL  string
	HTTP     *http.Client
	ClientID string
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("zero API returned %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

func NewClient(baseURL string, opts Options) *Client {
	httpClient := opts.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Minute}
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: httpClient, ClientID: opts.ClientID}
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.ClientID != "" {
		req.Header.Set("X-Zero-Client-ID", c.ClientID)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) Health(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodGet, "/health", nil, nil)
}
