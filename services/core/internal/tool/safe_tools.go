package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxReadBytes       = 512 * 1024
	maxToolOutputBytes = 256 * 1024
	maxScanTokenBytes  = 1024 * 1024
)

var defaultIgnoredDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	"coverage": true, ".next": true, "tmp": true, "vendor": true,
}

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
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Result{}, err
	}
	if info.IsDir() {
		return Result{}, fmt.Errorf("read path is a directory: %s", input.Path)
	}
	if info.Size() > maxReadBytes {
		return Result{}, fmt.Errorf("file too large to read safely: %s (%d bytes > %d bytes)", input.Path, info.Size(), maxReadBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Result{}, err
	}
	if isBinary(data) {
		return Result{}, fmt.Errorf("refusing to read binary file: %s", input.Path)
	}
	content := string(data)
	if input.StartLine > 0 || input.EndLine > 0 {
		if input.StartLine > 0 && input.EndLine > 0 && input.StartLine > input.EndLine {
			return Result{}, fmt.Errorf("invalid line range: startLine %d is after endLine %d", input.StartLine, input.EndLine)
		}
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
			return Result{Title: input.Path, Output: ""}, nil
		}
		content = strings.Join(lines[start-1:end], "\n")
	}
	return Result{Title: input.Path, Output: truncateOutput(content, maxToolOutputBytes)}, nil
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
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var lines []string
	if input.Recursive {
		projectRoot, err := filepath.Abs(tc.ProjectPath)
		if err != nil {
			return Result{}, err
		}
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if p != abs && d.IsDir() && defaultIgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(projectRoot, p)
			if err != nil {
				return nil
			}
			label := filepath.ToSlash(rel)
			if d.IsDir() {
				label += "/"
			}
			lines = append(lines, label)
			return nil
		})
		if err != nil {
			return Result{}, err
		}
	} else {
		entries, err := os.ReadDir(abs)
		if err != nil {
			return Result{}, err
		}
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
			lines = append(lines, fmt.Sprintf("%s%s\t%d", entry.Name(), suffix, size))
		}
	}
	sort.Strings(lines)
	return Result{Title: input.Path, Output: truncateOutput(strings.Join(lines, "\n"), maxToolOutputBytes)}, nil
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
	if input.MaxResults > 1000 {
		input.MaxResults = 1000
	}
	matcher, err := globMatcher(filepath.ToSlash(input.Pattern))
	if err != nil {
		return Result{}, err
	}
	var matches []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path != root && d.IsDir() && defaultIgnoredDirs[d.Name()] {
			return filepath.SkipDir
		}
		if len(matches) >= input.MaxResults {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if matcher(rel) {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	sort.Strings(matches)
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
	if input.MaxResults > 1000 {
		input.MaxResults = 1000
	}
	projectRoot, _ := filepath.Abs(tc.ProjectPath)
	var results []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && defaultIgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(results) >= input.MaxResults {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenBytes)
		lineNum := 0
		for scanner.Scan() && len(results) < input.MaxResults {
			lineNum++
			line := scanner.Text()
			if lineNum == 1 && isBinary([]byte(line)) {
				return nil
			}
			if re.MatchString(line) {
				rel, _ := filepath.Rel(projectRoot, path)
				results = append(results, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), lineNum, line))
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Title: input.Pattern, Output: strings.Join(results, "\n")}, nil
}

type walkTool struct{}

func Walk() Tool { return walkTool{} }

func (walkTool) Name() string          { return "walk" }
func (walkTool) Description() string   { return "Walk a project folder and return a bounded file tree" }
func (walkTool) NeedsPermission() bool { return false }
func (walkTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"maxDepth":{"type":"integer"},"maxFiles":{"type":"integer"},"includeHidden":{"type":"boolean"}}}`)
}

func (walkTool) Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error) {
	var input struct {
		Path          string `json:"path"`
		MaxDepth      int    `json:"maxDepth"`
		MaxFiles      int    `json:"maxFiles"`
		IncludeHidden bool   `json:"includeHidden"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &input); err != nil {
			return Result{}, err
		}
	}
	if input.Path == "" {
		input.Path = "."
	}
	if input.MaxDepth <= 0 {
		input.MaxDepth = 3
	}
	if input.MaxFiles <= 0 {
		input.MaxFiles = 200
	}

	root, err := scopedPath(tc.ProjectPath, input.Path)
	if err != nil {
		return Result{}, err
	}
	projectRoot, err := filepath.Abs(tc.ProjectPath)
	if err != nil {
		return Result{}, err
	}

	var lines []string
	dirs, files, total := 0, 0, 0
	truncated := false

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		name := d.Name()
		if path != root {
			if d.IsDir() && defaultIgnoredDirs[name] {
				return filepath.SkipDir
			}
			if !input.IncludeHidden && strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		relFromRoot, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		depth := 0
		if relFromRoot != "." {
			depth = strings.Count(relFromRoot, string(filepath.Separator)) + 1
		}
		if depth > input.MaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if total >= input.MaxFiles {
			truncated = true
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relProject, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}
		label := filepath.ToSlash(relProject)
		if relFromRoot == "." {
			label = filepath.ToSlash(input.Path)
		}
		if d.IsDir() {
			label += "/"
			dirs++
		} else {
			files++
		}
		lines = append(lines, strings.Repeat("  ", depth)+label)
		total++
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	sort.Strings(lines[1:])
	if truncated {
		lines = append(lines, fmt.Sprintf("… truncated at %d entries", input.MaxFiles))
	}
	lines = append(lines, fmt.Sprintf("Summary: %d dirs, %d files, maxDepth=%d", dirs, files, input.MaxDepth))
	return Result{Title: input.Path, Output: strings.Join(lines, "\n")}, nil
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	sample := data[:limit]
	if bytesIndex(sample, 0) >= 0 {
		return true
	}
	return !utf8.Valid(sample)
}

func bytesIndex(data []byte, b byte) int {
	for i, v := range data {
		if v == b {
			return i
		}
	}
	return -1
}

func truncateOutput(output string, maxBytes int) string {
	if maxBytes <= 0 || len(output) <= maxBytes {
		return output
	}
	return output[:maxBytes] + fmt.Sprintf("\n... truncated at %d bytes", maxBytes)
}

func globMatcher(pattern string) (func(string) bool, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("glob pattern required")
	}
	if !strings.Contains(pattern, "**") {
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, err
		}
		return func(rel string) bool {
			matched, _ := path.Match(pattern, rel)
			return matched
		}, nil
	}
	quoted := regexp.QuoteMeta(pattern)
	quoted = strings.ReplaceAll(quoted, `\*\*/`, `(?:.*/)?`)
	quoted = strings.ReplaceAll(quoted, `\*\*`, `.*`)
	quoted = strings.ReplaceAll(quoted, `\*`, `[^/]*`)
	quoted = strings.ReplaceAll(quoted, `\?`, `[^/]`)
	re, err := regexp.Compile("^" + quoted + "$")
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
}
