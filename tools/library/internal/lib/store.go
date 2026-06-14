package lib

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store is the on-disk library at root.
type Store struct {
	Root string
}

// New returns a Store rooted at the given directory.
func New(root string) *Store {
	return &Store{Root: root}
}

func (s *Store) entriesDir() string  { return filepath.Join(s.Root, "entries") }
func (s *Store) archiveDir() string  { return filepath.Join(s.Root, "archive") }
func (s *Store) indexesDir() string  { return filepath.Join(s.Root, "indexes") }
func (s *Store) indexFile() string   { return filepath.Join(s.indexesDir(), "index.json") }
func (s *Store) tagFile() string     { return filepath.Join(s.indexesDir(), "tags.json") }
func (s *Store) summaryFile() string { return filepath.Join(s.Root, "LIBRARY.md") }

// Init ensures the directory layout exists.
func (s *Store) Init() error {
	for _, d := range []string{s.entriesDir(), s.archiveDir(), s.indexesDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(s.indexFile()); errors.Is(err, os.ErrNotExist) {
		empty := Index{Version: 1, UpdatedAt: time.Now(), ByType: map[string]int{}, EntryIDs: []string{}}
		if err := writeJSON(s.indexFile(), empty); err != nil {
			return err
		}
	}
	if _, err := os.Stat(s.tagFile()); errors.Is(err, os.ErrNotExist) {
		empty := TagIndex{UpdatedAt: time.Now(), Tags: map[string][]string{}}
		if err := writeJSON(s.tagFile(), empty); err != nil {
			return err
		}
	}
	return nil
}

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

// entryPath returns the on-disk path for an entry by ID, archived or not.
func (s *Store) entryPath(id string, archived bool) string {
	dir := s.entriesDir()
	if archived {
		dir = s.archiveDir()
	}
	return filepath.Join(dir, id+".json")
}

// Put writes a new or updated entry. If e.ID is empty, one is generated.
func (s *Store) Put(e *Entry) error {
	if err := s.Init(); err != nil {
		return err
	}
	if !ValidType(e.Type) {
		return fmt.Errorf("invalid type: %q", e.Type)
	}
	if strings.TrimSpace(e.Title) == "" {
		return errors.New("title required")
	}
	now := time.Now().UTC()
	if e.ID == "" {
		e.ID = newID()
		e.CreatedAt = now
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
	if e.Confidence <= 0 {
		e.Confidence = 0.6
	}
	if e.Confidence > 1 {
		e.Confidence = 1
	}
	if e.Source == "" {
		e.Source = SourceAgent
	}
	e.Tags = normalizeTags(e.Tags)

	if err := writeJSON(s.entryPath(e.ID, e.Archived), *e); err != nil {
		return err
	}
	return s.Reindex()
}

// Get loads an entry by ID (checks active first, then archive).
func (s *Store) Get(id string) (*Entry, error) {
	for _, archived := range []bool{false, true} {
		p := s.entryPath(id, archived)
		if _, err := os.Stat(p); err == nil {
			var e Entry
			if err := readJSON(p, &e); err != nil {
				return nil, err
			}
			return &e, nil
		}
	}
	return nil, fmt.Errorf("entry not found: %s", id)
}

// List returns all entries, optionally including archived ones.
func (s *Store) List(includeArchived bool) ([]Entry, error) {
	entries, err := readDir(s.entriesDir())
	if err != nil {
		return nil, err
	}
	if includeArchived {
		arch, err := readDir(s.archiveDir())
		if err != nil {
			return nil, err
		}
		entries = append(entries, arch...)
	}
	return entries, nil
}

// Archive moves an entry from active to archive directory.
func (s *Store) Archive(id string) error {
	e, err := s.Get(id)
	if err != nil {
		return err
	}
	if e.Archived {
		return nil
	}
	src := s.entryPath(id, false)
	e.Archived = true
	e.UpdatedAt = time.Now().UTC()
	if err := writeJSON(s.entryPath(id, true), *e); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.Reindex()
}

// Restore moves an entry back from archive to active.
func (s *Store) Restore(id string) error {
	e, err := s.Get(id)
	if err != nil {
		return err
	}
	if !e.Archived {
		return nil
	}
	src := s.entryPath(id, true)
	e.Archived = false
	e.UpdatedAt = time.Now().UTC()
	if err := writeJSON(s.entryPath(id, false), *e); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.Reindex()
}

// IncrementUse bumps usage stats and (optionally) hit count for an entry.
func (s *Store) IncrementUse(id string, helped bool) error {
	e, err := s.Get(id)
	if err != nil {
		return err
	}
	e.UseCount++
	if helped {
		e.HitCount++
	}
	e.UpdatedAt = time.Now().UTC()
	return writeJSON(s.entryPath(id, e.Archived), *e)
}

// Reindex rebuilds index.json and tags.json by scanning the filesystem.
func (s *Store) Reindex() error {
	active, err := readDir(s.entriesDir())
	if err != nil {
		return err
	}
	archived, err := readDir(s.archiveDir())
	if err != nil {
		return err
	}

	idx := Index{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		ByType:    map[string]int{},
	}
	for _, e := range active {
		idx.EntryIDs = append(idx.EntryIDs, e.ID)
		idx.ByType[string(e.Type)]++
	}
	for _, e := range archived {
		idx.ArchivedID = append(idx.ArchivedID, e.ID)
	}
	idx.Total = len(active)
	sort.Strings(idx.EntryIDs)
	sort.Strings(idx.ArchivedID)

	tagIdx := TagIndex{UpdatedAt: idx.UpdatedAt, Tags: map[string][]string{}}
	for _, e := range active {
		for _, t := range e.Tags {
			tagIdx.Tags[t] = append(tagIdx.Tags[t], e.ID)
		}
	}
	for _, ids := range tagIdx.Tags {
		sort.Strings(ids)
	}

	if err := writeJSON(s.indexFile(), idx); err != nil {
		return err
	}
	if err := writeJSON(s.tagFile(), tagIdx); err != nil {
		return err
	}
	return s.WriteSummary(active)
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func readDir(dir string) ([]Entry, error) {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		var e Entry
		if err := readJSON(filepath.Join(dir, f.Name()), &e); err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name(), err)
		}
		out = append(out, e)
	}
	return out, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
