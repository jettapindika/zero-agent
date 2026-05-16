package tool_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zero-agent/core/internal/tool"
)

func testProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "readme.md"), []byte("hello zero\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	return dir
}

func TestReadToolReadsScopedFile(t *testing.T) {
	project := testProject(t)
	result, err := tool.Read().Execute(context.Background(), mustJSON(t, map[string]any{"path": "docs/readme.md"}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(result.Output, "hello zero") {
		t.Fatalf("expected file content, got %q", result.Output)
	}
}

func TestReadToolBlocksTraversal(t *testing.T) {
	project := testProject(t)
	_, err := tool.Read().Execute(context.Background(), mustJSON(t, map[string]any{"path": "../secret"}), tool.Context{ProjectPath: project})
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestLsToolListsDirectory(t *testing.T) {
	project := testProject(t)
	result, err := tool.Ls().Execute(context.Background(), mustJSON(t, map[string]any{"path": "."}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(result.Output, "main.go") || !strings.Contains(result.Output, "docs") {
		t.Fatalf("unexpected listing: %q", result.Output)
	}
}

func TestGlobToolFindsFiles(t *testing.T) {
	project := testProject(t)
	result, err := tool.Glob().Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "**/*.md"}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(result.Output, "docs/readme.md") {
		t.Fatalf("unexpected glob output: %q", result.Output)
	}
}

func TestGrepToolFindsMatches(t *testing.T) {
	project := testProject(t)
	result, err := tool.Grep().Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "hello", "path": "."}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(result.Output, "docs/readme.md:1") {
		t.Fatalf("unexpected grep output: %q", result.Output)
	}
}

func TestRegistryContainsDefaultTools(t *testing.T) {
	registry := tool.DefaultRegistry()
	for _, name := range []string{"read", "ls", "glob", "grep", "bash", "write", "edit", "fetch"} {
		if registry.Get(name) == nil {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	return data
}
