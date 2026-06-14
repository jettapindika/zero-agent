package filehandler

import (
	"path/filepath"
	"strings"
)

const (
	MIMEDocx = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MIMEXlsx = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MIMEPdf  = "application/pdf"
)

var extToMIME = map[string]string{
	".go":   "text/x-go",
	".ts":   "text/typescript",
	".tsx":  "text/typescript",
	".js":   "text/javascript",
	".jsx":  "text/javascript",
	".py":   "text/x-python",
	".rs":   "text/x-rust",
	".rb":   "text/x-ruby",
	".java": "text/x-java",
	".c":    "text/x-c",
	".h":    "text/x-c",
	".cpp":  "text/x-c++",
	".hpp":  "text/x-c++",
	".cs":   "text/x-csharp",
	".php":  "text/x-php",
	".swift": "text/x-swift",
	".kt":   "text/x-kotlin",
	".lua":  "text/x-lua",
	".md":   "text/markdown",
	".mdx":  "text/markdown",
	".txt":  "text/plain",
	".log":  "text/plain",
	".json": "application/json",
	".yaml": "text/yaml",
	".yml":  "text/yaml",
	".toml": "application/toml",
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".sql":  "text/x-sql",
	".sh":   "text/x-sh",
	".bash": "text/x-sh",
	".zsh":  "text/x-sh",
	".env":  "text/plain",
	".conf": "text/plain",
	".ini":  "text/plain",
	".csv":  "text/csv",
	".tsv":  "text/tab-separated-values",
	".xml":  "text/xml",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".pdf":  MIMEPdf,
	".docx": MIMEDocx,
	".xlsx": MIMEXlsx,
}

func DetectMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if m, ok := extToMIME[ext]; ok {
		return m
	}
	return "application/octet-stream"
}

func IsAllowed(mime string) bool {
	switch {
	case strings.HasPrefix(mime, "text/"):
		return true
	case strings.HasPrefix(mime, "image/"):
		return true
	case mime == MIMEPdf, mime == MIMEDocx, mime == MIMEXlsx:
		return true
	case mime == "application/json", mime == "application/toml", mime == "application/xml":
		return true
	}
	return false
}

func IsTextLike(mime string) bool {
	return strings.HasPrefix(mime, "text/") ||
		mime == "application/json" ||
		mime == "application/toml" ||
		mime == "application/xml"
}

func IsImage(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}
