package lib

import "time"

// EntryType is the kind of knowledge captured.
type EntryType string

const (
	TypeMistake    EntryType = "mistake"
	TypeInsight    EntryType = "insight"
	TypePattern    EntryType = "pattern"
	TypeConvention EntryType = "convention"
	TypeFix        EntryType = "fix"
)

// AllTypes returns every supported entry type.
func AllTypes() []EntryType {
	return []EntryType{TypeMistake, TypeInsight, TypePattern, TypeConvention, TypeFix}
}

// ValidType reports whether t is a known entry type.
func ValidType(t EntryType) bool {
	for _, v := range AllTypes() {
		if v == t {
			return true
		}
	}
	return false
}

// Source describes where an entry came from.
type Source string

const (
	SourceAgent          Source = "agent"
	SourceUserCorrection Source = "user-correction"
	SourceManual         Source = "manual"
	SourceRetry          Source = "retry"
)

// Entry is a single library record.
type Entry struct {
	ID         string    `json:"id"`
	Type       EntryType `json:"type"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Tags       []string  `json:"tags,omitempty"`
	FilePaths  []string  `json:"file_paths,omitempty"`
	Source     Source    `json:"source"`
	Confidence float64   `json:"confidence"` // 0..1
	UseCount   int       `json:"use_count"`
	HitCount   int       `json:"hit_count"` // times entry actually helped
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Archived   bool      `json:"archived,omitempty"`
}

// Score returns the ranking score used for injection ordering.
//
// Mistakes and conventions get a boost. Confidence is weighted by a
// helpful-rate (hit/use). New entries (use==0) fall back to confidence.
func (e Entry) Score() float64 {
	base := e.Confidence
	if e.UseCount > 0 {
		helpful := float64(e.HitCount) / float64(e.UseCount)
		base = 0.4*e.Confidence + 0.6*helpful
	}

	switch e.Type {
	case TypeMistake:
		base += 0.20
	case TypeConvention:
		base += 0.15
	case TypeFix:
		base += 0.05
	}

	if base < 0 {
		base = 0
	}
	if base > 1.5 {
		base = 1.5
	}
	return base
}

// Index is the on-disk index.json shape.
type Index struct {
	Version    int            `json:"version"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Total      int            `json:"total"`
	ByType     map[string]int `json:"by_type"`
	EntryIDs   []string       `json:"entry_ids"`
	ArchivedID []string       `json:"archived_ids,omitempty"`
}

// TagIndex maps tag -> entry IDs.
type TagIndex struct {
	UpdatedAt time.Time           `json:"updated_at"`
	Tags      map[string][]string `json:"tags"`
}
