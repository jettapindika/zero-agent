package agent

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var genericSessionTitles = map[string]bool{
	"":                true,
	"desktop session": true,
	"new session":     true,
	"untitled":        true,
	"untitled session": true,
}

var fencedCodeRE = regexp.MustCompile("(?s)```.*?```")
var inlineCodeRE = regexp.MustCompile("`[^`]*`")
var whitespaceRE = regexp.MustCompile(`\s+`)

func shouldAutoTitleSession(title string) bool {
	return genericSessionTitles[strings.ToLower(strings.TrimSpace(title))]
}

func titleFromPrompt(prompt string) string {
	cleaned := fencedCodeRE.ReplaceAllString(prompt, " ")
	cleaned = inlineCodeRE.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(whitespaceRE.ReplaceAllString(cleaned, " "))
	if cleaned == "" {
		return "New Task"
	}

	for _, separator := range []string{"\n", ". ", "? ", "! ", ": "} {
		if before, _, ok := strings.Cut(cleaned, separator); ok && strings.TrimSpace(before) != "" {
			cleaned = strings.TrimSpace(before)
			break
		}
	}

	words := strings.Fields(cleaned)
	if len(words) > 7 {
		words = words[:7]
	}
	title := strings.Join(words, " ")
	title = strings.Trim(title, " \t\n\r.,;:!?-_*#[](){}\"'")
	if utf8.RuneCountInString(title) > 60 {
		title = trimRunes(title, 60)
	}
	if looksEnglish(title) && !looksIndonesian(title) {
		title = titleCase(title)
	}
	if title == "" {
		return "New Task"
	}
	return title
}

func trimRunes(value string, limit int) string {
	var out strings.Builder
	for i, r := range value {
		if i >= limit {
			break
		}
		out.WriteRune(r)
	}
	return strings.TrimSpace(out.String())
}

func looksEnglish(value string) bool {
	letters := 0
	asciiLetters := 0
	for _, r := range value {
		if unicode.IsLetter(r) {
			letters++
			if r <= unicode.MaxASCII {
				asciiLetters++
			}
		}
	}
	return letters > 0 && asciiLetters == letters
}

func looksIndonesian(value string) bool {
	commonWords := map[string]bool{
		"buat": true, "yang": true, "dan": true, "di": true, "ke": true,
		"dari": true, "untuk": true, "dengan": true, "bisa": true, "ubah": true,
	}
	for _, word := range strings.Fields(strings.ToLower(value)) {
		if commonWords[word] {
			return true
		}
	}
	return false
}

func titleCase(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		lower := strings.ToLower(word)
		if len(lower) <= 2 && i > 0 {
			words[i] = lower
			continue
		}
		runes := []rune(lower)
		if len(runes) > 0 {
			runes[0] = unicode.ToTitle(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
