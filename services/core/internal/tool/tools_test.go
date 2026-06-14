package tool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestReadToolRejectsInvalidLineRange(t *testing.T) {
	project := testProject(t)
	_, err := tool.Read().Execute(context.Background(), mustJSON(t, map[string]any{"path": "docs/readme.md", "startLine": 3, "endLine": 1}), tool.Context{ProjectPath: project})
	if err == nil {
		t.Fatal("expected invalid line range error")
	}
}

func TestReadToolRejectsBinaryFiles(t *testing.T) {
	project := testProject(t)
	if err := os.WriteFile(filepath.Join(project, "image.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	_, err := tool.Read().Execute(context.Background(), mustJSON(t, map[string]any{"path": "image.bin"}), tool.Context{ProjectPath: project})
	if err == nil {
		t.Fatal("expected binary file error")
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

func TestLsToolListsRecursivelyWhenRequested(t *testing.T) {
	project := testProject(t)
	result, err := tool.Ls().Execute(context.Background(), mustJSON(t, map[string]any{"path": ".", "recursive": true}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("ls recursive: %v", err)
	}
	if !strings.Contains(result.Output, "docs/readme.md") {
		t.Fatalf("recursive listing missing nested file: %q", result.Output)
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

func TestGlobToolMatchesNestedPatternsPrecisely(t *testing.T) {
	project := testProject(t)
	if err := os.WriteFile(filepath.Join(project, "root.md"), []byte("root"), 0o644); err != nil {
		t.Fatalf("write root md: %v", err)
	}
	result, err := tool.Glob().Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "docs/*.md"}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(result.Output, "docs/readme.md") || strings.Contains(result.Output, "root.md") {
		t.Fatalf("unexpected scoped glob output: %q", result.Output)
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

func TestGrepToolSkipsIgnoredDirectories(t *testing.T) {
	project := testProject(t)
	if err := os.MkdirAll(filepath.Join(project, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir ignored: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "node_modules", "pkg", "index.js"), []byte("hello ignored\n"), 0o644); err != nil {
		t.Fatalf("write ignored: %v", err)
	}
	result, err := tool.Grep().Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "hello", "path": "."}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if strings.Contains(result.Output, "node_modules") {
		t.Fatalf("grep should skip ignored directory: %q", result.Output)
	}
}

func TestDangerousToolsValidateInputs(t *testing.T) {
	project := testProject(t)
	if _, err := tool.Write().Execute(context.Background(), mustJSON(t, map[string]any{"path": "", "content": "x"}), tool.Context{ProjectPath: project}); err == nil {
		t.Fatal("expected write path error")
	}
	if _, err := tool.Edit().Execute(context.Background(), mustJSON(t, map[string]any{"path": "docs/readme.md", "oldString": "", "newString": "x"}), tool.Context{ProjectPath: project}); err == nil {
		t.Fatal("expected edit oldString error")
	}
	if _, err := tool.Fetch().Execute(context.Background(), mustJSON(t, map[string]any{"url": "file:///etc/passwd"}), tool.Context{ProjectPath: project}); err == nil {
		t.Fatal("expected fetch scheme error")
	}
	if _, err := tool.Fetch().Execute(context.Background(), mustJSON(t, map[string]any{"url": "https://example.com", "method": "TRACE"}), tool.Context{ProjectPath: project}); err == nil {
		t.Fatal("expected fetch method error")
	}
}

func TestFetchToolReportsStatusAndContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	project := testProject(t)
	result, err := tool.Fetch().Execute(context.Background(), mustJSON(t, map[string]any{"url": server.URL}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(result.Output, "status=200") || !strings.Contains(result.Output, "content-type=text/plain") || !strings.Contains(result.Output, "ok") {
		t.Fatalf("unexpected fetch output: %q", result.Output)
	}
}

func TestRegistryContainsDefaultTools(t *testing.T) {
	registry := tool.DefaultRegistry()
	for _, name := range []string{"read", "ls", "glob", "grep", "walk", "bash", "write", "edit", "fetch"} {
		if registry.Get(name) == nil {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestDefaultToolSchemasAreValidJSON(t *testing.T) {
	registry := tool.DefaultRegistry()
	for _, name := range []string{"read", "ls", "glob", "grep", "walk", "bash", "write", "edit", "fetch"} {
		t.Run(name, func(t *testing.T) {
			var schema map[string]any
			if err := json.Unmarshal(registry.Get(name).Schema(), &schema); err != nil {
				t.Fatalf("invalid schema: %v", err)
			}
			if schema["type"] != "object" {
				t.Fatalf("schema type should be object: %#v", schema)
			}
		})
	}
}

func TestWalkToolListsBoundedTree(t *testing.T) {
	project := testProject(t)
	if err := os.MkdirAll(filepath.Join(project, "internal", "agent"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "internal", "agent", "runner.go"), []byte("package agent\n"), 0o644); err != nil {
		t.Fatalf("write nested: %v", err)
	}

	result, err := tool.Walk().Execute(context.Background(), mustJSON(t, map[string]any{"path": ".", "maxDepth": 3}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for _, want := range []string{"./", "docs/", "docs/readme.md", "internal/", "internal/agent/", "internal/agent/runner.go", "Summary:"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("walk output missing %q:\n%s", want, result.Output)
		}
	}
}

func TestWalkToolHonorsMaxDepth(t *testing.T) {
	project := testProject(t)
	if err := os.MkdirAll(filepath.Join(project, "a", "b"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "a", "b", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatalf("write deep: %v", err)
	}

	result, err := tool.Walk().Execute(context.Background(), mustJSON(t, map[string]any{"path": ".", "maxDepth": 1}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if strings.Contains(result.Output, "deep.txt") {
		t.Fatalf("walk should omit deep file:\n%s", result.Output)
	}
}

func TestWalkToolHonorsMaxFiles(t *testing.T) {
	project := testProject(t)
	result, err := tool.Walk().Execute(context.Background(), mustJSON(t, map[string]any{"path": ".", "maxFiles": 2}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if !strings.Contains(result.Output, "truncated at 2 entries") {
		t.Fatalf("walk should report truncation:\n%s", result.Output)
	}
}

func TestWalkToolSkipsIgnoredAndHiddenDirs(t *testing.T) {
	project := testProject(t)
	if err := os.MkdirAll(filepath.Join(project, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "node_modules", "pkg", "index.js"), []byte("module"), 0o644); err != nil {
		t.Fatalf("write ignored: %v", err)
	}
	if err := os.Mkdir(filepath.Join(project, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".hidden", "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}

	result, err := tool.Walk().Execute(context.Background(), mustJSON(t, map[string]any{"path": ".", "maxDepth": 3}), tool.Context{ProjectPath: project})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for _, notWant := range []string{"node_modules", ".hidden", "secret.txt"} {
		if strings.Contains(result.Output, notWant) {
			t.Fatalf("walk output should skip %q:\n%s", notWant, result.Output)
		}
	}
}

func TestWalkToolBlocksTraversal(t *testing.T) {
	project := testProject(t)
	_, err := tool.Walk().Execute(context.Background(), mustJSON(t, map[string]any{"path": ".."}), tool.Context{ProjectPath: project})
	if err == nil {
		t.Fatal("expected traversal error")
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
