package lib

import (
	"fmt"
	"sort"
	"strings"
)

// MaxInject is the hard cap on entries injected into a session prompt.
const MaxInject = 8

// Inject returns up to MaxInject entries ordered by Score, with mistakes
// guaranteed at the front. Archived entries are excluded.
func (s *Store) Inject() ([]Entry, error) {
	all, err := s.List(false)
	if err != nil {
		return nil, err
	}

	mistakes := make([]Entry, 0)
	others := make([]Entry, 0)
	for _, e := range all {
		if e.Type == TypeMistake {
			mistakes = append(mistakes, e)
		} else {
			others = append(others, e)
		}
	}
	sort.Slice(mistakes, func(i, j int) bool { return mistakes[i].Score() > mistakes[j].Score() })
	sort.Slice(others, func(i, j int) bool { return others[i].Score() > others[j].Score() })

	out := append([]Entry{}, mistakes...)
	out = append(out, others...)
	if len(out) > MaxInject {
		out = out[:MaxInject]
	}
	return out, nil
}

// RenderInjection produces the markdown block to embed in a system prompt.
func RenderInjection(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<!-- BEGIN agent-library -->\n")
	b.WriteString("## Agent Library (learned context)\n\n")
	b.WriteString("These are high-priority lessons from prior sessions. Treat MISTAKE entries as hard rules.\n\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "- **[%s]** %s\n", strings.ToUpper(string(e.Type)), e.Title)
		if strings.TrimSpace(e.Body) != "" {
			body := strings.TrimSpace(e.Body)
			body = strings.ReplaceAll(body, "\n", "\n  ")
			fmt.Fprintf(&b, "  %s\n", body)
		}
		if len(e.FilePaths) > 0 {
			fmt.Fprintf(&b, "  _files: %s_\n", strings.Join(e.FilePaths, ", "))
		}
	}
	b.WriteString("<!-- END agent-library -->\n")
	return b.String()
}

// AutoArchive moves entries below threshold (with enough usage data) to archive.
// Returns IDs that were archived.
func (s *Store) AutoArchive(minUseCount int, threshold float64) ([]string, error) {
	all, err := s.List(false)
	if err != nil {
		return nil, err
	}
	var archived []string
	for _, e := range all {
		if e.UseCount < minUseCount {
			continue
		}
		if e.Score() < threshold {
			if err := s.Archive(e.ID); err != nil {
				return archived, err
			}
			archived = append(archived, e.ID)
		}
	}
	return archived, nil
}
