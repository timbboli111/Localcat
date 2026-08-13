package group

import (
	"testing"
)

func TestGUIStateUpsertGroup(t *testing.T) {
	s := NewGUIState()
	g, _ := Create("Test Group", "host-1", Open)
	g.Code = "12345678"

	s.UpsertGroup(g, 3, "Hello")

	conv, exists := s.Get("group:" + g.ID)
	if !exists {
		t.Fatal("Get() not found")
	}
	if conv.Title != "Test Group" {
		t.Fatalf("Title = %q, want Test Group", conv.Title)
	}
	if conv.UnreadCount != 3 {
		t.Fatalf("UnreadCount = %d, want 3", conv.UnreadCount)
	}
	if conv.LastMessage != "Hello" {
		t.Fatalf("LastMessage = %q, want Hello", conv.LastMessage)
	}
	if !conv.IsGroup {
		t.Fatal("IsGroup = false, want true")
	}
}

func TestGUIStateUpsertDirect(t *testing.T) {
	s := NewGUIState()
	s.UpsertDirect("peer-1", "Peer One", 1, "Hi")

	conv, exists := s.Get("direct:peer-1")
	if !exists {
		t.Fatal("Get() not found")
	}
	if conv.IsGroup {
		t.Fatal("IsGroup = true, want false")
	}
	if conv.Title != "Peer One" {
		t.Fatalf("Title = %q, want Peer One", conv.Title)
	}
}

func TestGUIStateMarkRead(t *testing.T) {
	s := NewGUIState()
	g, _ := Create("Test Group", "host-1", Open)
	s.UpsertGroup(g, 5, "Hello")

	s.MarkRead("group:" + g.ID)
	conv, _ := s.Get("group:" + g.ID)
	if conv.UnreadCount != 0 {
		t.Fatalf("UnreadCount after MarkRead = %d, want 0", conv.UnreadCount)
	}
}

func TestGUIStateSetUnread(t *testing.T) {
	s := NewGUIState()
	g, _ := Create("Test Group", "host-1", Open)
	s.UpsertGroup(g, 0, "")

	s.SetUnread("group:"+g.ID, 7)
	conv, _ := s.Get("group:" + g.ID)
	if conv.UnreadCount != 7 {
		t.Fatalf("UnreadCount after SetUnread = %d, want 7", conv.UnreadCount)
	}
}

func TestGUIStateRemove(t *testing.T) {
	s := NewGUIState()
	g, _ := Create("Test Group", "host-1", Open)
	s.UpsertGroup(g, 0, "")

	s.Remove("group:" + g.ID)
	if _, exists := s.Get("group:" + g.ID); exists {
		t.Fatal("Get() after Remove still exists")
	}
}

func TestGUIStateAll(t *testing.T) {
	s := NewGUIState()
	g1, _ := Create("Alpha", "host-1", Open)
	g2, _ := Create("Beta", "host-2", Open)
	s.UpsertGroup(g1, 0, "")
	s.UpsertGroup(g2, 0, "")
	s.UpsertDirect("peer-1", "Charlie", 0, "")

	all := s.All()
	if len(all) != 3 {
		t.Fatalf("All() len = %d, want 3", len(all))
	}
	if all[0].Title != "Alpha" || all[1].Title != "Beta" || all[2].Title != "Charlie" {
		t.Fatalf("All() not sorted: %v, %v, %v", all[0].Title, all[1].Title, all[2].Title)
	}
}

func TestFormatGroupCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"97845201", "97845201"},
		{"9784 5201", "97845201"},
		{"9784-5201", "97845201"},
		{"9784520", ""},
		{"978452011", ""},
		{"abcdefgh", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := FormatGroupCode(tt.input)
		if got != tt.expected {
			t.Fatalf("FormatGroupCode(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidateGroupCodeInput(t *testing.T) {
	if err := ValidateGroupCodeInput("97845201"); err != nil {
		t.Fatalf("ValidateGroupCodeInput(valid) error = %v", err)
	}
	if err := ValidateGroupCodeInput("123"); err == nil {
		t.Fatal("ValidateGroupCodeInput(short) succeeded")
	}
	if err := ValidateGroupCodeInput("abcdefgh"); err == nil {
		t.Fatal("ValidateGroupCodeInput(non-digit) succeeded")
	}
}
