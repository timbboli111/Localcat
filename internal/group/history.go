package group

import (
	"fmt"
	"strings"

	"localcat/internal/history"
)

// HistoryAdapter bridges GroupMessage to the existing history.Store.
type HistoryAdapter struct {
	store *history.Store
}

// NewHistoryAdapter creates a HistoryAdapter backed by the given store.
func NewHistoryAdapter(store *history.Store) *HistoryAdapter {
	return &HistoryAdapter{store: store}
}

// SaveIncoming persists an incoming group message.
// Returns an error if the message is invalid or a duplicate.
func (a *HistoryAdapter) SaveIncoming(g *Group, msg GroupMessage) error {
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	if err := ValidateGroupMessage(msg); err != nil {
		return err
	}
	if a.store.HasMessageID(msg.MessageID) {
		return fmt.Errorf("duplicate message id %q", msg.MessageID)
	}

	conv := history.Conversation{
		ID:        history.GroupID(msg.GroupID),
		Type:      history.GroupConversation,
		Title:     g.Name,
		UpdatedAt: msg.Timestamp,
	}
	hmsg := history.Message{
		MessageID:  msg.MessageID,
		SenderID:   msg.SenderID,
		SenderName: msg.SenderName,
		Text:       msg.Body,
		SentAt:     msg.Timestamp,
		Outgoing:   false,
	}
	return a.store.AddMessage(conv, hmsg)
}

// SaveOutgoing persists an outgoing group message.
// Returns an error if the message is invalid or a duplicate.
func (a *HistoryAdapter) SaveOutgoing(g *Group, msg GroupMessage) error {
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	if err := ValidateGroupMessage(msg); err != nil {
		return err
	}
	if a.store.HasMessageID(msg.MessageID) {
		return fmt.Errorf("duplicate message id %q", msg.MessageID)
	}

	conv := history.Conversation{
		ID:        history.GroupID(msg.GroupID),
		Type:      history.GroupConversation,
		Title:     g.Name,
		UpdatedAt: msg.Timestamp,
	}
	hmsg := history.Message{
		MessageID:  msg.MessageID,
		SenderID:   msg.SenderID,
		SenderName: msg.SenderName,
		Text:       msg.Body,
		SentAt:     msg.Timestamp,
		Outgoing:   true,
	}
	return a.store.AddMessage(conv, hmsg)
}

// TrimBody trims whitespace from a message body.
func TrimBody(s string) string {
	return strings.TrimSpace(s)
}
