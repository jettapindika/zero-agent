package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return s
}

func TestPutAndGetRoundtrip(t *testing.T) {
	s := newStore(t)
	e := &Entry{
		Type:       TypeMistake,
		Title:      "no x/oauth2 when net/http stdlib",
		Body:       "use stdlib",
		Tags:       []string{" Go ", "go", ""},
		Confidence: 0.9,
	}
	if err := s.Put(e); err != nil {
		t.Fatalf("put: %v", err)
	}
	if e.ID == "" {
		t.Fatal("expected ID generated")
	}
	got, err := s.Get(e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != e.Title {
		t.Errorf("title mismatch")
	}
	if len(got.Tags) != 1 || got.Tags[0] != "go" {
		t.Errorf("tags not normalized: %#v", got.Tags)
	}
	if got.Source != SourceAgent {
		t.Errorf("default source not applied: %s", got.Source)
	}
}

func TestScoreBoosts(t *testing.T) {
	mistake := Entry{Type: TypeMistake, Confidence: 0.5}
	insight := Entry{Type: TypeInsight, Confidence: 0.5}
	if mistake.Score() <= insight.Score() {
		t.Errorf("mistake should outrank insight at equal confidence: m=%.2f i=%.2f",
			mistake.Score(), insight.Score())
	}

	helpful := Entry{Type: TypeInsight, Confidence: 0.5, UseCount: 10, HitCount: 9}
	unhelpful := Entry{Type: TypeInsight, Confidence: 0.5, UseCount: 10, HitCount: 0}
	if helpful.Score() <= unhelpful.Score() {
		t.Errorf("helpful entry should score higher")
	}
}

func TestInjectMistakesFirstAndCap(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 12; i++ {
		typ := TypeInsight
		if i%4 == 0 {
			typ = TypeMistake
		}
		e := &Entry{Type: typ, Title: "x", Confidence: 0.5}
		if err := s.Put(e); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	got, err := s.Inject()
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if len(got) > MaxInject {
		t.Errorf("inject exceeded MaxInject: %d", len(got))
	}
	mistakeCount := 0
	for _, e := range got {
		if e.Type == TypeMistake {
			mistakeCount++
		}
	}
	for i := 0; i < mistakeCount; i++ {
		if got[i].Type != TypeMistake {
			t.Errorf("position %d should be mistake, got %s", i, got[i].Type)
		}
	}
}

func TestArchiveExcludedFromInject(t *testing.T) {
	s := newStore(t)
	e := &Entry{Type: TypeMistake, Title: "archived", Confidence: 0.99}
	_ = s.Put(e)
	if err := s.Archive(e.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, _ := s.Inject()
	for _, x := range got {
		if x.ID == e.ID {
			t.Errorf("archived entry should not be injected")
		}
	}
	if _, err := s.Get(e.ID); err != nil {
		t.Errorf("archived entry should be restorable via Get: %v", err)
	}
}

func TestAutoArchive(t *testing.T) {
	s := newStore(t)
	low := &Entry{Type: TypeInsight, Title: "low", Confidence: 0.2}
	_ = s.Put(low)
	for i := 0; i < 6; i++ {
		_ = s.IncrementUse(low.ID, false)
	}
	high := &Entry{Type: TypeInsight, Title: "high", Confidence: 0.9}
	_ = s.Put(high)
	for i := 0; i < 6; i++ {
		_ = s.IncrementUse(high.ID, true)
	}

	ids, err := s.AutoArchive(5, 0.30)
	if err != nil {
		t.Fatalf("auto-archive: %v", err)
	}
	if len(ids) != 1 || ids[0] != low.ID {
		t.Errorf("expected only low entry archived, got %v", ids)
	}
}

func TestSummaryRegenerates(t *testing.T) {
	s := newStore(t)
	_ = s.Put(&Entry{Type: TypeMistake, Title: "first lesson", Confidence: 0.9})
	path := filepath.Join(s.Root, "LIBRARY.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("LIBRARY.md not generated: %v", err)
	}
	if !strings.Contains(string(b), "first lesson") {
		t.Errorf("LIBRARY.md missing entry title; got:\n%s", b)
	}
	if !strings.Contains(string(b), "Total entries: 1") {
		t.Errorf("LIBRARY.md missing total; got:\n%s", b)
	}
}

func TestRenderInjectionFormat(t *testing.T) {
	out := RenderInjection([]Entry{
		{Type: TypeMistake, Title: "no x/oauth2", Body: "use net/http"},
	})
	if !strings.Contains(out, "BEGIN agent-library") {
		t.Errorf("missing begin marker")
	}
	if !strings.Contains(out, "[MISTAKE]") {
		t.Errorf("missing type tag")
	}
	if !strings.Contains(out, "use net/http") {
		t.Errorf("missing body")
	}
}
