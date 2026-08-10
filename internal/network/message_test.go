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
