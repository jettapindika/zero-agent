package upload

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxRequestBytes = 100 << 20

type Store struct {
	Root string
}

func NewStore(root string) *Store {
	return &Store{Root: root}
}

func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "zero-uploads")
	}
	return filepath.Join(home, ".zero", "uploads")
}

func (s *Store) sessionDir(sessionID string) (string, error) {
	if !validID(sessionID) {
		return "", fmt.Errorf("invalid session id")
	}
	return filepath.Join(s.Root, sessionID), nil
}

func (s *Store) Save(sessionID, attachmentID, ext string, src io.Reader) (string, int64, error) {
	dir, err := s.sessionDir(sessionID)
	if err != nil {
		return "", 0, err
	}
	if !validID(attachmentID) {
		return "", 0, fmt.Errorf("invalid attachment id")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", 0, fmt.Errorf("mkdir uploads dir: %w", err)
	}

	cleanExt := sanitizeExt(ext)
	path := filepath.Join(dir, attachmentID+cleanExt)
	tmp := path + ".tmp"

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("create %s: %w", tmp, err)
	}
	n, copyErr := io.Copy(f, src)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return "", 0, fmt.Errorf("copy upload: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmp)
		return "", 0, fmt.Errorf("close upload: %w", closeErr)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", 0, fmt.Errorf("rename upload: %w", err)
	}
	return path, n, nil
}

func (s *Store) Remove(path string) error {
	if path == "" {
		return nil
	}
	if !strings.HasPrefix(path, s.Root) {
		return errors.New("refusing to remove path outside store root")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) RemoveSession(sessionID string) error {
	dir, err := s.sessionDir(sessionID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func validID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func sanitizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if len(ext) > 16 {
		return ""
	}
	for _, r := range ext[1:] {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9':
		default:
			return ""
		}
	}
	return ext
}
