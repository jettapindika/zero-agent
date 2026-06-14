package filehandler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name string, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}

func TestDetectMIME(t *testing.T) {
	cases := map[string]string{
		"x.go":   "text/x-go",
		"x.md":   "text/markdown",
		"x.json": "application/json",
		"x.PDF":  MIMEPdf,
		"x.docx": MIMEDocx,
		"x.xlsx": MIMEXlsx,
		"x.png":  "image/png",
		"x.bin":  "application/octet-stream",
	}
	for name, want := range cases {
		if got := DetectMIME(name); got != want {
			t.Errorf("DetectMIME(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIsAllowedAndHelpers(t *testing.T) {
	if !IsAllowed("text/markdown") {
		t.Error("text/markdown should be allowed")
	}
	if !IsAllowed("image/png") {
		t.Error("image/png should be allowed")
	}
	if !IsAllowed(MIMEPdf) || !IsAllowed(MIMEDocx) || !IsAllowed(MIMEXlsx) {
		t.Error("office formats should be allowed")
	}
	if IsAllowed("application/x-executable") {
		t.Error("executables must not be allowed")
	}
	if !IsTextLike("application/json") || !IsTextLike("text/yaml") {
		t.Error("json + yaml should be text-like")
	}
	if IsTextLike("image/png") {
		t.Error("image/png is not text")
	}
	if !IsImage("image/jpeg") {
		t.Error("jpeg should be image")
	}
}

func TestLoadFileSmallText(t *testing.T) {
	path := writeTemp(t, "hello.md", []byte("# hello\nworld\n"))
	lf, err := LoadFile(path, "hello.md")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if lf.IsChunked {
		t.Error("small file should not be chunked")
	}
	if !strings.Contains(lf.Content, "world") {
		t.Errorf("content missing: %q", lf.Content)
	}
	if lf.MIMEType != "text/markdown" {
		t.Errorf("mime = %q", lf.MIMEType)
	}
	if lf.ChunkCount() != 1 {
		t.Errorf("ChunkCount = %d, want 1", lf.ChunkCount())
	}
}

func TestLoadFileChunksLargeText(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10000; i++ {
		b.WriteString("the quick brown fox jumps over the lazy dog\n")
	}
	path := writeTemp(t, "big.txt", []byte(b.String()))

	lf, err := LoadFile(path, "big.txt")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if !lf.IsChunked {
		t.Errorf("large file should be chunked (size=%d)", lf.Size)
	}
	if len(lf.Chunks) < 2 {
		t.Errorf("expected >=2 chunks, got %d", len(lf.Chunks))
	}
	rejoined := strings.Join(lf.Chunks, "")
	if !strings.HasPrefix(rejoined, "the quick brown fox") {
		t.Error("rejoined chunks should preserve content prefix")
	}
}

func TestLoadFileImageEncodesBase64(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0xDE, 0xAD, 0xBE, 0xEF}
	path := writeTemp(t, "tiny.png", pngHeader)

	lf, err := LoadFile(path, "tiny.png")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if !lf.IsBinary {
		t.Error("image should be marked binary")
	}
	if lf.Base64 == "" {
		t.Error("Base64 should be populated for image")
	}
	if !strings.HasPrefix(TextPlaceholderForImage(lf), "[Image attached:") {
		t.Errorf("placeholder format unexpected: %q", TextPlaceholderForImage(lf))
	}
}

func TestLoadFileRejectsUnsupported(t *testing.T) {
	path := writeTemp(t, "thing.bin", []byte{0x00, 0x01, 0x02})
	_, err := LoadFile(path, "thing.bin")
	if err == nil {
		t.Fatal("expected unsupported error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention unsupported: %v", err)
	}
}

func TestLoadFileRejectsEmpty(t *testing.T) {
	path := writeTemp(t, "empty.md", []byte{})
	_, err := LoadFile(path, "empty.md")
	if err != ErrEmptyFile {
		t.Errorf("expected ErrEmptyFile, got %v", err)
	}
}

func TestChunkTextSmallFitsInOne(t *testing.T) {
	chunks := ChunkText("a\nb\nc\n", 1000)
	if len(chunks) != 1 {
		t.Errorf("small input should give 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "a") || !strings.Contains(chunks[0], "c") {
		t.Errorf("chunk content lost: %q", chunks[0])
	}
}

func TestChunkTextSplitsAtBoundary(t *testing.T) {
	parts := ChunkText("aaaa\nbbbb\ncccc\n", 5)
	if len(parts) < 2 {
		t.Errorf("expected multiple chunks for tiny budget, got %d: %#v", len(parts), parts)
	}
}
