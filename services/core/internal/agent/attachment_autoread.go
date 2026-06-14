package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/zero-agent/core/internal/tool"
)

var attachmentIDRegex = regexp.MustCompile(`- id=([a-f0-9\-]{36})`)

type attachmentInfo struct {
	ID       string
	OrigName string
	MIMEType string
}

func isAttachmentSystemMessage(text string) bool {
	return strings.Contains(text, "attach_read") &&
		strings.Contains(text, "id=")
}

func extractAttachmentIDs(text string) []string {
	matches := attachmentIDRegex.FindAllStringSubmatch(text, -1)
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			ids = append(ids, m[1])
		}
	}
	return ids
}

func autoReadAttachments(
	ctx context.Context,
	store tool.AttachStore,
	ids []string,
) ([]tool.Result, []attachmentInfo, error) {
	results := make([]tool.Result, 0, len(ids))
	infos := make([]attachmentInfo, 0, len(ids))

	for _, id := range ids {
		att, err := store.GetAttachment(ctx, id)
		if err != nil {
			continue
		}

		args, _ := json.Marshal(map[string]any{"id": id})
		readTool := tool.AttachRead(store)
		result, err := readTool.Execute(ctx, args, tool.Context{SessionID: att.SessionID})
		if err != nil {
			continue
		}

		results = append(results, result)
		infos = append(infos, attachmentInfo{
			ID:       att.ID,
			OrigName: att.OrigName,
			MIMEType: att.MIMEType,
		})
	}

	return results, infos, nil
}

func buildAckMessage(info attachmentInfo) string {
	if strings.HasPrefix(info.MIMEType, "image/") {
		return fmt.Sprintf("📎 Gambar diterima — %s. Silakan tanya apa saja.", info.OrigName)
	}
	if info.MIMEType == "application/pdf" {
		return fmt.Sprintf("📎 PDF dibaca — %s. Siap digunakan.", info.OrigName)
	}
	if strings.HasPrefix(info.MIMEType, "text/") {
		return fmt.Sprintf("📎 File dibaca — %s. Siap digunakan.", info.OrigName)
	}
	return fmt.Sprintf("📎 File diterima — %s (%s).", info.OrigName, info.MIMEType)
}
