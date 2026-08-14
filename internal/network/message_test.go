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

	if err := WriteMessage(&buf, Message{Type: "group", GroupID: "g", Body: "hello"}); err == nil {
		t.Fatal("WriteMessage() group with empty sender id succeeded")
	}

	if err := WriteMessage(&buf, Message{Type: "group", SenderID: "s", Body: "hello"}); err == nil {
		t.Fatal("WriteMessage() group with empty group id succeeded")
	}

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

func TestJoinRequestMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	wantTime := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	want := Message{
		Type:          "join_request",
		GroupID:       "group-123",
		RequesterID:   "requester-1",
		RequesterName: "Requester One",
		Time:          wantTime,
	}
	if err := WriteMessage(&buf, want); err != nil {
		t.Fatalf("WriteMessage() join_request error = %v", err)
	}
	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage() join_request error = %v", err)
	}
	if got.Type != "join_request" {
		t.Fatalf("Type = %q, want join_request", got.Type)
	}
	if got.GroupID != "group-123" {
		t.Fatalf("GroupID = %q, want group-123", got.GroupID)
	}
	if got.RequesterID != "requester-1" {
		t.Fatalf("RequesterID = %q, want requester-1", got.RequesterID)
	}
	if got.RequesterName != "Requester One" {
		t.Fatalf("RequesterName = %q, want Requester One", got.RequesterName)
	}
	if !got.Time.Equal(wantTime) {
		t.Fatalf("Time = %v, want %v", got.Time, wantTime)
	}
}

func TestWriteMessageValidatesJoinRequestFields(t *testing.T) {
	var buf bytes.Buffer

	if err := WriteMessage(&buf, Message{Type: "join_request", GroupID: "g"}); err == nil {
		t.Fatal("WriteMessage() join_request with empty requester id succeeded")
	}

	if err := WriteMessage(&buf, Message{Type: "join_request", RequesterID: "r"}); err == nil {
		t.Fatal("WriteMessage() join_request with empty group id succeeded")
	}
}

func TestJoinAcceptMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := Message{
		Type:        "join_accept",
		GroupID:     "group-123",
		RequesterID: "requester-1",
		Time:        time.Now(),
	}
	if err := WriteMessage(&buf, want); err != nil {
		t.Fatalf("WriteMessage() join_accept error = %v", err)
	}
	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage() join_accept error = %v", err)
	}
	if got.Type != "join_accept" {
		t.Fatalf("Type = %q, want join_accept", got.Type)
	}
}

func TestJoinRejectMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := Message{
		Type:        "join_reject",
		GroupID:     "group-123",
		RequesterID: "requester-1",
		Reason:      "Not approved",
		Time:        time.Now(),
	}
	if err := WriteMessage(&buf, want); err != nil {
		t.Fatalf("WriteMessage() join_reject error = %v", err)
	}
	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage() join_reject error = %v", err)
	}
	if got.Type != "join_reject" {
		t.Fatalf("Type = %q, want join_reject", got.Type)
	}
	if got.Reason != "Not approved" {
		t.Fatalf("Reason = %q, want Not approved", got.Reason)
	}
}
