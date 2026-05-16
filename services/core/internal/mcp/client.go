package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

type ServerConfig struct {
	Type    Transport `json:"type"`
	Command string    `json:"command,omitempty"`
	Args    []string  `json:"args,omitempty"`
	URL     string    `json:"url,omitempty"`
}

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Client struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	seq    int
}

func NewStdioClient(name, command string, args []string, workDir string) (*Client, error) {
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
		return nil, fmt.Errorf("start mcp %s: %w", name, err)
	}
	return &Client{name: name, cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}, nil
}

func (c *Client) Name() string { return c.name }

func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

func (c *Client) Initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "zero-agent", "version": "0.1.0"},
	}
	_, err := c.call(ctx, "initialize", params)
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized", nil)
}

func (c *Client) ListTools(ctx context.Context) ([]ToolDef, error) {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Tools []ToolDef `json:"tools"`
	}
	json.Unmarshal(result, &resp)
	return resp.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	params := map[string]any{"name": name, "arguments": json.RawMessage(args)}
	return c.call(ctx, "tools/call", params)
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.seq++
	id := c.seq
	c.mu.Unlock()

	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if err := c.send(msg); err != nil {
		return nil, err
	}
	return c.readResponse(id)
}

func (c *Client) notify(method string, params any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
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
	for {
		header, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		var contentLen int
		fmt.Sscanf(header, "Content-Length: %d", &contentLen)
		if contentLen == 0 {
			continue
		}
		c.stdout.ReadString('\n')
		body := make([]byte, contentLen)
		if _, err := io.ReadFull(c.stdout, body); err != nil {
			return nil, err
		}
		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

type MCPTool struct {
	client  *Client
	toolDef ToolDef
}

func (t *MCPTool) Name() string            { return t.client.Name() + ":" + t.toolDef.Name }
func (t *MCPTool) Description() string     { return t.toolDef.Description }
func (t *MCPTool) Schema() json.RawMessage { return t.toolDef.InputSchema }
func (t *MCPTool) NeedsPermission() bool   { return true }

func (t *MCPTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	return t.client.CallTool(ctx, t.toolDef.Name, args)
}

func ToolsFromClient(client *Client, tools []ToolDef) []*MCPTool {
	result := make([]*MCPTool, 0, len(tools))
	for _, td := range tools {
		result = append(result, &MCPTool{client: client, toolDef: td})
	}
	return result
}

func ParseConfig(data []byte) (map[string]ServerConfig, error) {
	var cfg struct {
		MCP map[string]ServerConfig `json:"mcp"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg.MCP, nil
}

func StartFromConfig(configs map[string]ServerConfig, workDir string) ([]*Client, error) {
	var clients []*Client
	for name, cfg := range configs {
		if cfg.Type != TransportStdio {
			continue
		}
		client, err := NewStdioClient(name, cfg.Command, cfg.Args, workDir)
		if err != nil {
			for _, c := range clients {
				c.Close()
			}
			return nil, fmt.Errorf("start mcp %s: %w", name, err)
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func init() {
	_ = bytes.Compare
}
