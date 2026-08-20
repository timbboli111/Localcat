package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"localcat/internal/group"
)

// Message is the JSON payload exchanged over TCP between LocalCat peers.
// The Type field distinguishes direct chat from group chat and control messages.
type Message struct {
	Type   string    `json:"type,omitempty"`    // "direct" (default), "group", "join_request", "join_accept", "join_reject"
	From   string    `json:"from"`              // Display name of sender
	FromID string    `json:"from_id,omitempty"` // Identity ID of sender
	Text   string    `json:"text"`              // Direct chat message body
	Time   time.Time `json:"time"`              // Timestamp

	// Group message fields
	MessageID  string           `json:"message_id,omitempty"`
	GroupID    string           `json:"group_id,omitempty"`
	GroupName  string           `json:"group_name,omitempty"`
	GroupCode  string           `json:"group_code,omitempty"`
	JoinPolicy group.JoinPolicy `json:"join_policy,omitempty"`
	SenderID   string           `json:"sender_id,omitempty"`
	SenderName string           `json:"sender_name,omitempty"`
	Body       string           `json:"body,omitempty"`

	// Join request/response fields
	RequesterID   string `json:"requester_id,omitempty"`
	RequesterName string `json:"requester_name,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// WriteMessage writes a single newline-delimited JSON message.
func WriteMessage(w io.Writer, msg Message) error {
	msg.Text = strings.TrimSpace(msg.Text)
	msg.Body = strings.TrimSpace(msg.Body)

	if msg.Type == "" {
		msg.Type = "direct"
	}

	switch msg.Type {
	case "direct":
		if msg.From == "" {
			return fmt.Errorf("sender name is required")
		}
		if msg.Text == "" {
			return fmt.Errorf("message text is required")
		}
	case "group":
		if msg.SenderID == "" {
			return fmt.Errorf("sender id is required")
		}
		if msg.GroupID == "" {
			return fmt.Errorf("group id is required")
		}
		if msg.Body == "" {
			return fmt.Errorf("message body is required")
		}
	case "join_request":
		if msg.RequesterID == "" {
			return fmt.Errorf("requester id is required")
		}
		if msg.GroupID == "" {
			return fmt.Errorf("group id is required")
		}
	case "join_accept":
		if msg.GroupID == "" {
			return fmt.Errorf("group id is required")
		}
		if msg.RequesterID == "" {
			return fmt.Errorf("requester id is required")
		}
	case "join_reject":
		if msg.GroupID == "" {
			return fmt.Errorf("group id is required")
		}
		if msg.RequesterID == "" {
			return fmt.Errorf("requester id is required")
		}
	default:
		return fmt.Errorf("unknown message type: %q", msg.Type)
	}

	if msg.Time.IsZero() {
		msg.Time = time.Now()
	}

	encoded, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	encoded = append(encoded, '\n')
	_, err = w.Write(encoded)
	if err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	return nil
}

// ReadMessage reads one newline-delimited JSON message.
func ReadMessage(r *bufio.Reader) (Message, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return Message{}, err
	}
	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return Message{}, fmt.Errorf("decode message: %w", err)
	}
	if msg.Type == "" {
		msg.Type = "direct"
	}
	return msg, nil
}
