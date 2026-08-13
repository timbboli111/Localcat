package network

import (
	"bufio"
	"bytes"
	"testing"
	"time"
)

func TestMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	wantTime := time.Date(2026, 8, 9, 10, 11, 12, 0, time.UTC)
	want := Message{From: "Alice", Text: "Halo LocalCat", Time: wantTime}
	if err := WriteMessage(&buf, want); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if got.From != want.From || got.Text != want.Text || !got.Time.Equal(want.Time) {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, want)
	}
	if got.Type != "direct" {
		t.Fatalf("Type = %q, want direct", got.Type)
	}
}

func TestWriteMessageValidatesRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMessage(&buf, Message{Text: "halo"}); err == nil {
		t.Fatal("WriteMessage() with empty sender succeeded")
	}
	if err := WriteMessage(&buf, Message{From: "Alice", Text: "   "}); err == nil {
		t.Fatal("WriteMessage() with empty text succeeded")
	}
}

func TestGroupMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	wantTime := time.Date(2026, 8, 9, 11, 12, 13, 0, time.UTC)
	want := Message{
		Type:       "group",
		MessageID:  "msg-123",
		GroupID:    "group-456",
		SenderID:   "sender-789",
		SenderName: "Sender Name",
		Body:       "Hello group",
		Time:       wantTime,
	}
	if err := WriteMessage(&buf, want); err != nil {
		t.Fatalf("WriteMessage() group error = %v", err)
	}
	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage() group error = %v", err)
	}
	if got.Type != "group" {
		t.Fatalf("Type = %q, want group", got.Type)
	}
	if got.MessageID != "msg-123" {
		t.Fatalf("MessageID = %q, want msg-123", got.MessageID)
	}
	if got.GroupID != "group-456" {
		t.Fatalf("GroupID = %q, want group-456", got.GroupID)
	}
	if got.SenderID != "sender-789" {
		t.Fatalf("SenderID = %q, want sender-789", got.SenderID)
	}
	if got.SenderName != "Sender Name" {
		t.Fatalf("SenderName = %q, want Sender Name", got.SenderName)
	}
	if got.Body != "Hello group" {
		t.Fatalf("Body = %q, want Hello group", got.Body)
	}
	if !got.Time.Equal(wantTime) {
		t.Fatalf("Time = %v, want %v", got.Time, wantTime)
	}
}

func TestWriteMessageValidatesGroupFields(t *testing.T) {
	var buf bytes.Buffer

	// Missing sender id
	if err := WriteMessage(&buf, Message{Type: "group", GroupID: "g", Body: "hello"}); err == nil {
		t.Fatal("WriteMessage() group with empty sender id succeeded")
	}

	// Missing group id
	if err := WriteMessage(&buf, Message{Type: "group", SenderID: "s", Body: "hello"}); err == nil {
		t.Fatal("WriteMessage() group with empty group id succeeded")
	}

	// Missing body
	if err := WriteMessage(&buf, Message{Type: "group", SenderID: "s", GroupID: "g", Body: "   "}); err == nil {
		t.Fatal("WriteMessage() group with empty body succeeded")
	}
}

func TestWriteMessageUnknownType(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMessage(&buf, Message{Type: "unknown", From: "Alice", Text: "hello"}); err == nil {
		t.Fatal("WriteMessage() with unknown type succeeded")
	}
}
