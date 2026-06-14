package filehandler

import (
	"encoding/base64"
	"fmt"
	"os"
)

func loadImage(lf *LoadedFile) (*LoadedFile, error) {
	if lf.Size > MaxImageBytes {
		return nil, fmt.Errorf("%w: image %s is %d bytes (max %d)",
			ErrFileTooLarge, lf.OrigName, lf.Size, MaxImageBytes)
	}
	raw, err := os.ReadFile(lf.Path)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", lf.Path, err)
	}
	lf.Base64 = base64.StdEncoding.EncodeToString(raw)
	lf.IsBinary = true
	return lf, nil
}

func TextPlaceholderForImage(lf *LoadedFile) string {
	return fmt.Sprintf(
		"[Image attached: %s, %d bytes, mime=%s. Vision rendering will be added in Phase 2; for now describe what you need from this image and the user can convey it.]",
		lf.OrigName, lf.Size, lf.MIMEType,
	)
}
