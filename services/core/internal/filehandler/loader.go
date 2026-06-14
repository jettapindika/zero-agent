package filehandler

import (
	"fmt"
	"os"
	"strings"
)

func LoadFile(absPath, origName string) (*LoadedFile, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", absPath, err)
	}
	if info.Size() == 0 {
		return nil, ErrEmptyFile
	}

	name := origName
	if name == "" {
		name = info.Name()
	}

	mime := DetectMIME(name)
	if !IsAllowed(mime) {
		return nil, fmt.Errorf("%w: %s (mime=%s)", ErrUnsupportedType, name, mime)
	}

	lf := &LoadedFile{
		Path:     absPath,
		OrigName: name,
		MIMEType: mime,
		Size:     info.Size(),
	}

	switch {
	case IsTextLike(mime):
		return loadText(lf)
	case IsImage(mime):
		return loadImage(lf)
	case mime == MIMEPdf:
		return lf, nil
	case mime == MIMEDocx:
		return lf, nil
	case mime == MIMEXlsx:
		return lf, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, mime)
}

func loadText(lf *LoadedFile) (*LoadedFile, error) {
	raw, err := os.ReadFile(lf.Path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", lf.Path, err)
	}
	content := string(raw)

	if int64(len(raw)) <= MaxInlineBytes {
		lf.Content = content
		return lf, nil
	}
	lf.IsChunked = true
	lf.Chunks = ChunkText(content, ChunkChars)
	return lf, nil
}

func ChunkText(text string, maxChars int) []string {
	if maxChars <= 0 {
		return []string{text}
	}
	var chunks []string
	lines := strings.Split(text, "\n")
	var buf strings.Builder
	count := 0
	for _, line := range lines {
		lineLen := len(line) + 1
		if count > 0 && count+lineLen > maxChars {
			chunks = append(chunks, buf.String())
			buf.Reset()
			count = 0
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
		count += lineLen
	}
	if buf.Len() > 0 {
		chunks = append(chunks, buf.String())
	}
	return chunks
}
