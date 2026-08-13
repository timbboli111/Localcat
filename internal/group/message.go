package group

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MessageType identifies the type of a network message.
type MessageType string

const (
	// MessageTypeDirect is the existing direct chat message.
	MessageTypeDirect MessageType = "direct"
	// MessageTypeGroup is a group chat message relayed by the Host.
	MessageTypeGroup MessageType = "group"
)

// GroupMessage is the payload for a GROUP_MESSAGE network message.
type GroupMessage struct {
	MessageID  string    `json:"message_id"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Body       string    `json:"body"`
	Timestamp  time.Time `json:"timestamp"`
}

// NewMessageID generates a unique 128-bit message identifier.
func NewMessageID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate message id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// ValidateGroupMessage validates a group message for correctness.
// It does NOT check group membership — that is the Host relay's job.
func ValidateGroupMessage(msg GroupMessage) error {
	if msg.MessageID == "" {
		return fmt.Errorf("message id is required")
	}
	if msg.GroupID == "" {
		return fmt.Errorf("group id is required")
	}
	if strings.TrimSpace(msg.SenderID) == "" {
		return fmt.Errorf("sender id is required")
	}
	if strings.TrimSpace(msg.SenderName) == "" {
		return fmt.Errorf("sender name is required")
	}
	if strings.TrimSpace(msg.Body) == "" {
		return fmt.Errorf("message body is required")
	}
	if msg.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	return nil
}
