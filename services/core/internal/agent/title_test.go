package agent

import "testing"

func TestTitleFromPromptEnglish(t *testing.T) {
	got := titleFromPrompt("fix the auth bug in login flow")
	want := "Fix The Auth Bug in Login Flow"
	if got != want {
		t.Fatalf("titleFromPrompt = %q, want %q", got, want)
	}
}

func TestTitleFromPromptIndonesianKeepsNaturalCase(t *testing.T) {
	got := titleFromPrompt("buat chat field sticky di bawah")
	want := "buat chat field sticky di bawah"
	if got != want {
		t.Fatalf("titleFromPrompt = %q, want %q", got, want)
	}
}

func TestTitleFromPromptStripsCodeBlocks(t *testing.T) {
	got := titleFromPrompt("```ts\nthrow new Error()\n```\nfix this crash")
	want := "Fix This Crash"
	if got != want {
		t.Fatalf("titleFromPrompt = %q, want %q", got, want)
	}
}

func TestShouldAutoTitleSession(t *testing.T) {
	if !shouldAutoTitleSession("Desktop session") {
		t.Fatal("expected generic desktop title to be auto-titled")
	}
	if shouldAutoTitleSession("OAuth Refactor") {
		t.Fatal("expected manual title to be preserved")
	}
}
