package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
)

// Compactor summarizes old messages to reduce context size.
type Compactor struct {
	db       *storage.DB
	provider provider.Provider
}

func NewCompactor(db *storage.DB, p provider.Provider) *Compactor {
	return &Compactor{db: db, provider: p}
}

// ShouldCompact returns true if session messages exceed threshold.
func (c *Compactor) ShouldCompact(ctx context.Context, sessionID string, maxMessages int) (bool, error) {
	messages, err := c.db.ListMessages(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return len(messages) > maxMessages, nil
}

// Compact summarizes older messages into a single system message, keeping recent ones intact.
func (c *Compactor) Compact(ctx context.Context, sessionID string, keepRecent int) error {
	messages, err := c.db.ListMessages(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(messages) <= keepRecent {
		return nil
	}

	// Build text from old messages to summarize
	oldMessages := messages[:len(messages)-keepRecent]
	var oldText strings.Builder
	for _, msg := range oldMessages {
		parts, err := c.db.ListParts(ctx, msg.ID)
		if err != nil {
			continue
		}
		for _, part := range parts {
			if part.Text != nil {
				fmt.Fprintf(&oldText, "[%s] %s\n", msg.Role, *part.Text)
			}
		}
	}

	// Ask provider to summarize
	summary, err := c.provider.GenerateText(ctx, provider.ChatRequest{
		Model: "cx/gpt-5.5",
		Messages: []provider.Message{
			{Role: "system", Content: "Summarize the following conversation concisely, preserving key decisions, file paths, and context needed to continue the work."},
			{Role: "user", Content: oldText.String()},
		},
	})
	if err != nil {
		return fmt.Errorf("compaction summary failed: %w", err)
	}

	// Delete old messages
	for _, msg := range oldMessages {
		_ = c.db.DeleteMessage(ctx, msg.ID)
	}

	// Insert summary as system message at the beginning
	summaryMsg, err := c.db.CreateMessage(ctx, sessionID, "system")
	if err != nil {
		return err
	}
	summaryText := "[Compacted context]\n" + summary.Text
	_, err = c.db.CreatePart(ctx, storage.CreatePartInput{
		MessageID: summaryMsg.ID,
		Type:      "text",
		OrderNum:  0,
		Text:      &summaryText,
	})
	return err
}
