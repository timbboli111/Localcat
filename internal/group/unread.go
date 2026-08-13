package group

import (
	"fmt"

	"localcat/internal/history"
)

// UnreadService tracks unread counts for group conversations.
// It delegates to the existing history.Store unread mechanism.
type UnreadService struct {
	store *history.Store
}

// NewUnreadService creates an UnreadService backed by a history.Store.
func NewUnreadService(store *history.Store) *UnreadService {
	return &UnreadService{store: store}
}

// GetUnreadCount returns the unread count for a group conversation.
func (u *UnreadService) GetUnreadCount(groupID string) int {
	convID := history.GroupID(groupID)
	conversations := u.store.Conversations()
	for _, c := range conversations {
		if c.ID == convID {
			return c.UnreadCount
		}
	}
	return 0
}

// MarkAsRead marks a group conversation as read.
func (u *UnreadService) MarkAsRead(groupID string) error {
	convID := history.GroupID(groupID)
	return u.store.MarkAsRead(convID)
}

// IncrementUnread is called when an incoming group message is persisted.
// This is handled automatically by history.Store.AddMessage, so this
// method is a no-op placeholder for API clarity.
func (u *UnreadService) IncrementUnread(groupID string) error {
	// Unread increment is handled by history.Store.AddMessage.
	// This method exists for API completeness and future use.
	return nil
}

// HasConversation checks whether a group conversation exists.
func (u *UnreadService) HasConversation(groupID string) bool {
	convID := history.GroupID(groupID)
	for _, c := range u.store.Conversations() {
		if c.ID == convID {
			return true
		}
	}
	return false
}

// GetConversationID returns the conversation ID for a group.
func GetConversationID(groupID string) string {
	return history.GroupID(groupID)
}

// ValidateGroupMessageForUnread ensures the message is valid for unread tracking.
func ValidateGroupMessageForUnread(msg GroupMessage) error {
	if msg.GroupID == "" {
		return fmt.Errorf("group id is required")
	}
	if msg.MessageID == "" {
		return fmt.Errorf("message id is required")
	}
	return nil
}
