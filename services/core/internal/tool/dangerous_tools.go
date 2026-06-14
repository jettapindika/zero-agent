package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxCommandOutputBytes = 256 * 1024
	maxFetchOutputBytes   = 512 * 1024
	maxDangerousTimeout   = 2 * time.Minute
)

type bashTool struct{}

func Bash() Tool { return bashTool{} }

func (bashTool) Name() string          { return "bash" }
func (bashTool) Description() string   { return "Run shell command" }
func (bashTool) NeedsPermission() bool { return true }
func (bashTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"}},"required":["command"]}`)
}

func (bashTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	timeout := time.Duration(input.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > maxDangerousTimeout {
		timeout = maxDangerousTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", input.Command)
	cmd.Dir = tc.ProjectPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return Result{}, err
		}
	}
	output := fmt.Sprintf("exit=%d duration=%dms\n--- stdout ---\n%s", exitCode, duration, truncateOutput(stdout.String(), maxCommandOutputBytes))
	if stderr.Len() > 0 {
		output += fmt.Sprintf("\n--- stderr ---\n%s", truncateOutput(stderr.String(), maxCommandOutputBytes))
	}
	if cmdCtx.Err() == context.DeadlineExceeded {
		output += fmt.Sprintf("\ncommand timed out after %s", timeout)
		return Result{Title: input.Command, Output: output, IsError: true}, nil
	}
	return Result{Title: input.Command, Output: output, IsError: exitCode != 0}, nil
}

type writeTool struct{}

func Write() Tool { return writeTool{} }

func (writeTool) Name() string          { return "write" }
func (writeTool) Description() string   { return "Write or overwrite file" }
func (writeTool) NeedsPermission() bool { return true }
func (writeTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}

func (writeTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	if input.Path == "" {
		return Result{}, fmt.Errorf("path required")
	}
	abs, err := scopedPath(tc.ProjectPath, input.Path)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(abs, []byte(input.Content), 0o644); err != nil {
		return Result{}, err
	}
	lines := strings.Count(input.Content, "\n")
	return Result{Title: input.Path, Output: fmt.Sprintf("wrote %d bytes, %d lines", len(input.Content), lines)}, nil
}

type editTool struct{}

func Edit() Tool { return editTool{} }

func (editTool) Name() string          { return "edit" }
func (editTool) Description() string   { return "Targeted string replacement" }
func (editTool) NeedsPermission() bool { return true }
func (editTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"oldString":{"type":"string"},"newString":{"type":"string"},"global":{"type":"boolean"}},"required":["path","oldString","newString"]}`)
}

func (editTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Path      string `json:"path"`
		OldString string `json:"oldString"`
		NewString string `json:"newString"`
		Global    bool   `json:"global"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	if input.Path == "" {
		return Result{}, fmt.Errorf("path required")
	}
	if input.OldString == "" {
		return Result{}, fmt.Errorf("oldString required")
	}
	abs, err := scopedPath(tc.ProjectPath, input.Path)
	if err != nil {
		return Result{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Result{}, err
	}
	content := string(data)
	count := strings.Count(content, input.OldString)
	if count == 0 {
		return Result{}, fmt.Errorf("oldString not found in %s", input.Path)
	}
	if count > 1 && !input.Global {
		return Result{}, fmt.Errorf("found %d matches in %s, set global=true to replace all", count, input.Path)
	}
	var updated string
	if input.Global {
		updated = strings.ReplaceAll(content, input.OldString, input.NewString)
	} else {
		updated = strings.Replace(content, input.OldString, input.NewString, 1)
	}
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return Result{}, err
	}
	return Result{Title: input.Path, Output: fmt.Sprintf("replaced %d occurrence(s)", count)}, nil
}

type fetchTool struct{}

func Fetch() Tool { return fetchTool{} }

func (fetchTool) Name() string          { return "fetch" }
func (fetchTool) Description() string   { return "HTTP GET/POST" }
func (fetchTool) NeedsPermission() bool { return true }
func (fetchTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"method":{"type":"string"},"body":{"type":"string"},"timeout":{"type":"integer"}},"required":["url"]}`)
}

func (fetchTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		URL     string `json:"url"`
		Method  string `json:"method"`
		Body    string `json:"body"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	if input.Method == "" {
		input.Method = "GET"
	}
	input.Method = strings.ToUpper(input.Method)
	switch input.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
	default:
		return Result{}, fmt.Errorf("unsupported HTTP method: %s", input.Method)
	}
	parsed, err := url.Parse(input.URL)
	if err != nil {
		return Result{}, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Result{}, fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return Result{}, fmt.Errorf("URL host required")
	}
	timeout := time.Duration(input.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if timeout > maxDangerousTimeout {
		timeout = maxDangerousTimeout
	}
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if input.Body != "" {
		bodyReader = strings.NewReader(input.Body)
	}
	req, err := http.NewRequestWithContext(fetchCtx, input.Method, input.URL, bodyReader)
	if err != nil {
		return Result{}, err
	}
	if input.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchOutputBytes+1))
	if err != nil {
		return Result{}, err
	}
	body := string(data)
	if len(data) > maxFetchOutputBytes {
		body = truncateOutput(body, maxFetchOutputBytes)
	}
	output := fmt.Sprintf("status=%d content-type=%s\n%s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	return Result{Title: input.URL, Output: output, IsError: resp.StatusCode >= 400}, nil
}
