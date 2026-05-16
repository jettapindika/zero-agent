package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type readTool struct{}

func Read() Tool { return readTool{} }

func (readTool) Name() string          { return "read" }
func (readTool) Description() string   { return "Read file content" }
func (readTool) NeedsPermission() bool { return false }
func (readTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"startLine":{"type":"integer"},"endLine":{"type":"integer"}},"required":["path"]}`)
}

func (readTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Path      string `json:"path"`
		StartLine int    `json:"startLine"`
		EndLine   int    `json:"endLine"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
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
	if input.StartLine > 0 || input.EndLine > 0 {
		lines := strings.Split(content, "\n")
		start := input.StartLine
		if start < 1 {
			start = 1
		}
		end := input.EndLine
		if end < 1 || end > len(lines) {
			end = len(lines)
		}
		if start > len(lines) {
			start = len(lines)
		}
		content = strings.Join(lines[start-1:end], "\n")
	}
	return Result{Title: input.Path, Output: content}, nil
}

type lsTool struct{}

func Ls() Tool { return lsTool{} }

func (lsTool) Name() string          { return "ls" }
func (lsTool) Description() string   { return "List directory entries" }
func (lsTool) NeedsPermission() bool { return false }
func (lsTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"recursive":{"type":"boolean"}},"required":["path"]}`)
}

func (lsTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	abs, err := scopedPath(tc.ProjectPath, input.Path)
	if err != nil {
		return Result{}, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return Result{}, err
	}
	var sb strings.Builder
	for _, entry := range entries {
		info, _ := entry.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		fmt.Fprintf(&sb, "%s%s\t%d\n", entry.Name(), suffix, size)
	}
	return Result{Title: input.Path, Output: sb.String()}, nil
}

type globTool struct{}

func Glob() Tool { return globTool{} }

func (globTool) Name() string          { return "glob" }
func (globTool) Description() string   { return "Find files by pattern" }
func (globTool) NeedsPermission() bool { return false }
func (globTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"maxResults":{"type":"integer"}},"required":["pattern"]}`)
}

func (globTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Pattern    string `json:"pattern"`
		MaxResults int    `json:"maxResults"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	root, err := filepath.Abs(tc.ProjectPath)
	if err != nil {
		return Result{}, err
	}
	if input.MaxResults <= 0 {
		input.MaxResults = 100
	}
	var matches []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || len(matches) >= input.MaxResults {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(root, path)
		matched, _ := filepath.Match(input.Pattern, filepath.Base(path))
		if !matched && strings.Contains(input.Pattern, "**") {
			parts := strings.SplitN(input.Pattern, "**/", 2)
			if len(parts) == 2 {
				matched, _ = filepath.Match(parts[1], filepath.Base(path))
			}
		}
		if matched && !d.IsDir() {
			matches = append(matches, rel)
		}
		return nil
	})
	return Result{Title: input.Pattern, Output: strings.Join(matches, "\n")}, nil
}

type grepTool struct{}

func Grep() Tool { return grepTool{} }

func (grepTool) Name() string          { return "grep" }
func (grepTool) Description() string   { return "Search file content with regex" }
func (grepTool) NeedsPermission() bool { return false }
func (grepTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"maxResults":{"type":"integer"}},"required":["pattern"]}`)
}

func (grepTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		MaxResults int    `json:"maxResults"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return Result{}, err
	}
	if input.Path == "" {
		input.Path = "."
	}
	root, err := scopedPath(tc.ProjectPath, input.Path)
	if err != nil {
		return Result{}, err
	}
	re, err := regexp.Compile(input.Pattern)
	if err != nil {
		return Result{}, fmt.Errorf("invalid regex: %w", err)
	}
	if input.MaxResults <= 0 {
		input.MaxResults = 50
	}
	projectRoot, _ := filepath.Abs(tc.ProjectPath)
	var results []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(results) >= input.MaxResults {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() && len(results) < input.MaxResults {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				rel, _ := filepath.Rel(projectRoot, path)
				results = append(results, fmt.Sprintf("%s:%d:%s", rel, lineNum, line))
			}
		}
		return nil
	})
	return Result{Title: input.Pattern, Output: strings.Join(results, "\n")}, nil
}
