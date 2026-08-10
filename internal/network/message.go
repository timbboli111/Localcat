package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Message is the JSON payload exchanged over TCP between LocalCat peers.
type Message struct {
	From   string    `json:"from"`
	FromID string    `json:"from_id,omitempty"`
	Text   string    `json:"text"`
	Time   time.Time `json:"time"`
}

// WriteMessage writes a single newline-delimited JSON message.
func WriteMessage(w io.Writer, msg Message) error {
	msg.Text = strings.TrimSpace(msg.Text)
	if msg.From == "" {
		return fmt.Errorf("sender name is required")
	}
	if msg.Text == "" {
		return fmt.Errorf("message text is required")
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
	return msg, nil
}
