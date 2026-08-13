package group

import (
	"strings"
	"testing"
	"time"
)

func TestNewMessageID(t *testing.T) {
	id1, err := NewMessageID()
	if err != nil {
		t.Fatalf("NewMessageID() error = %v", err)
	}
	id2, err := NewMessageID()
	if err != nil {
		t.Fatalf("NewMessageID() error = %v", err)
	}
	if len(id1) != 32 {
		t.Fatalf("MessageID length = %d, want 32", len(id1))
	}
	if id1 == id2 {
		t.Fatal("two generated MessageIDs are identical")
	}
	for _, c := range id1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("MessageID %q contains invalid character %q", id1, c)
		}
	}
}

func TestValidateGroupMessage(t *testing.T) {
	valid := GroupMessage{
		MessageID:  "msg-1",
		GroupID:    "group-1",
		SenderID:   "sender-1",
		SenderName: "Sender One",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}
	if err := ValidateGroupMessage(valid); err != nil {
		t.Fatalf("ValidateGroupMessage(valid) error = %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(m *GroupMessage)
		wantErr string
	}{
		{"empty message id", func(m *GroupMessage) { m.MessageID = "" }, "message id"},
		{"empty group id", func(m *GroupMessage) { m.GroupID = "" }, "group id"},
		{"empty sender id", func(m *GroupMessage) { m.SenderID = "" }, "sender id"},
		{"empty sender name", func(m *GroupMessage) { m.SenderName = "" }, "sender name"},
		{"empty body", func(m *GroupMessage) { m.Body = "   " }, "message body"},
		{"zero timestamp", func(m *GroupMessage) { m.Timestamp = time.Time{} }, "timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid
			tt.mutate(&m)
			err := ValidateGroupMessage(m)
			if err == nil {
				t.Fatalf("ValidateGroupMessage() succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateGroupMessage() error = %q, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestMessageTypeConstants(t *testing.T) {
	if MessageTypeDirect != "direct" {
		t.Fatalf("MessageTypeDirect = %q, want direct", MessageTypeDirect)
	}
	if MessageTypeGroup != "group" {
		t.Fatalf("MessageTypeGroup = %q, want group", MessageTypeGroup)
	}
}
