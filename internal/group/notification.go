package group

import (
	"fmt"
	"strings"
)

// Notifier is the interface for sending notifications.
// The existing Fyne app notification mechanism implements this.
type Notifier interface {
	SendNotification(title, content string)
}

// NotificationService bridges group messages to the notification system.
type NotificationService struct {
	notifier Notifier
	store    *HistoryAdapter
}

// NewNotificationService creates a NotificationService.
func NewNotificationService(notifier Notifier, historyAdapter *HistoryAdapter) *NotificationService {
	return &NotificationService{
		notifier: notifier,
		store:    historyAdapter,
	}
}

// NotifyIncoming sends a notification for an incoming group message.
// It validates the message first and does not notify for duplicates.
func (n *NotificationService) NotifyIncoming(g *Group, msg GroupMessage) error {
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	if err := ValidateGroupMessage(msg); err != nil {
		return err
	}
	if n.store.store.HasMessageID(msg.MessageID) {
		return fmt.Errorf("duplicate message id %q", msg.MessageID)
	}
	if g.Closed {
		return fmt.Errorf("group is closed")
	}
	if !g.HasMember(msg.SenderID) {
		return fmt.Errorf("sender %q not a member", msg.SenderID)
	}

	title := g.Name
	content := fmt.Sprintf("%s: %s", msg.SenderName, truncate(msg.Body, 80))
	n.notifier.SendNotification(title, content)
	return nil
}

// NotifyOutgoing does nothing for the sender; outgoing messages
// should not produce a notification to self.
func (n *NotificationService) NotifyOutgoing(g *Group, msg GroupMessage) error {
	// Outgoing messages do not trigger notifications.
	return nil
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
