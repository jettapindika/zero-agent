package agent

import (
	"context"
	"strings"

	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
)

type Runner struct {
	db       *storage.DB
	bus      *bus.Bus
	provider provider.Provider
}

func NewRunner(db *storage.DB, eventBus *bus.Bus, p provider.Provider) *Runner {
	return &Runner{db: db, bus: eventBus, provider: p}
}

func (r *Runner) Run(ctx context.Context, sessionID string) error {
	session, err := r.db.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	messages, err := r.db.ListMessages(ctx, sessionID)
	if err != nil {
		return err
	}

	providerMessages := make([]provider.Message, 0, len(messages))
	for _, message := range messages {
		parts, err := r.db.ListParts(ctx, message.ID)
		if err != nil {
			return err
		}
		var content strings.Builder
		for _, part := range parts {
			if part.Text != nil {
				content.WriteString(*part.Text)
			}
		}
		providerMessages = append(providerMessages, provider.Message{Role: message.Role, Content: content.String()})
	}

	stream, err := r.provider.StreamChat(ctx, provider.ChatRequest{Model: session.Model, Messages: providerMessages})
	if err != nil {
		return err
	}

	assistantMessage, err := r.db.CreateMessage(ctx, session.ID, "assistant")
	if err != nil {
		return err
	}
	r.bus.Publish("message.created", session.ProjectID, session.ID, map[string]any{"message": assistantMessage, "parts": []storage.Part{}})

	var text strings.Builder
	for event := range stream {
		if event.Err != nil {
			return event.Err
		}
		if event.Delta == "" {
			continue
		}
		text.WriteString(event.Delta)
		r.bus.Publish("part.delta", session.ProjectID, session.ID, map[string]string{"messageId": assistantMessage.ID, "delta": event.Delta})
	}

	finalText := text.String()
	part, err := r.db.CreatePart(ctx, storage.CreatePartInput{MessageID: assistantMessage.ID, Type: "text", OrderNum: 0, Text: &finalText})
	if err != nil {
		return err
	}
	r.bus.Publish("part.created", session.ProjectID, session.ID, part)
	r.bus.Publish("session.status", session.ProjectID, session.ID, map[string]string{"status": "idle"})
	return nil
}
