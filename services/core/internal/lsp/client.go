package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
	seq    int
}

type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func NewClient(command string, args []string, workDir string) (*Client, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start lsp %s: %w", command, err)
	}
	return &Client{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	params := map[string]any{
		"processId":    nil,
		"rootUri":      rootURI,
		"capabilities": map[string]any{},
	}
	_, err := c.request(ctx, "initialize", params)
	if err != nil {
		return err
	}
	return c.notify("initialized", map[string]any{})
}

func (c *Client) GetDiagnostics(ctx context.Context, file string) ([]Diagnostic, error) {
	params := map[string]any{
		"textDocument": map[string]string{"uri": "file://" + file},
	}
	result, err := c.request(ctx, "textDocument/diagnostic", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			Range struct {
				Start struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"start"`
			} `json:"range"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
		} `json:"items"`
	}
	json.Unmarshal(result, &resp)
	diagnostics := make([]Diagnostic, 0, len(resp.Items))
	for _, item := range resp.Items {
		sev := "info"
		switch item.Severity {
		case 1:
			sev = "error"
		case 2:
			sev = "warning"
		case 3:
			sev = "info"
		case 4:
			sev = "hint"
		}
		diagnostics = append(diagnostics, Diagnostic{
			File:     file,
			Line:     item.Range.Start.Line + 1,
			Col:      item.Range.Start.Character + 1,
			Severity: sev,
			Message:  item.Message,
		})
	}
	return diagnostics, nil
}

func (c *Client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.seq++
	id := c.seq
	c.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if err := c.send(msg); err != nil {
		return nil, err
	}
	return c.readResponse(id)
}

func (c *Client) notify(method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return c.send(msg)
}

func (c *Client) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return err
	}
	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) readResponse(id int) (json.RawMessage, error) {
	buf := make([]byte, 4096)
	var accumulated bytes.Buffer
	for {
		n, err := c.stdout.Read(buf)
		if err != nil {
			return nil, err
		}
		accumulated.Write(buf[:n])
		content := accumulated.Bytes()
		idx := bytes.Index(content, []byte("\r\n\r\n"))
		if idx < 0 {
			continue
		}
		body := content[idx+4:]
		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		if resp.ID != id {
			accumulated.Reset()
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("lsp error: %s", resp.Error.Message)
		}
		return resp.Result, nil
	}
}
