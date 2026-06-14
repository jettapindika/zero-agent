package filehandler

import "errors"

const (
	MaxInlineBytes = 200_000

	ChunkChars = 32_000

	MaxImageBytes = 20 * 1024 * 1024
)

var (
	ErrUnsupportedType = errors.New("unsupported file type")
	ErrFileTooLarge    = errors.New("file too large")
	ErrEmptyFile       = errors.New("empty file")
)

type LoadedFile struct {
	Path      string
	OrigName  string
	MIMEType  string
	Size      int64
	Content   string
	Chunks    []string
	IsChunked bool
	IsBinary  bool
	Base64    string
}

func (l *LoadedFile) ChunkCount() int {
	if l.IsChunked {
		return len(l.Chunks)
	}
	if l.Content == "" {
		return 0
	}
	return 1
}
