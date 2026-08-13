package group

import (
	"fmt"
	"testing"
	"time"
)

func TestUnreadServiceGetUnreadCount(t *testing.T) {
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	unreadService := NewUnreadService(store)

	g, _ := Create("Test Group", "host-1", Open)

	// Initially 0
	if got := unreadService.GetUnreadCount(g.ID); got != 0 {
		t.Fatalf("initial unread = %d, want 0", got)
	}

	// Save incoming
	msg := GroupMessage{
		MessageID:  "msg-unread-svc-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	if got := unreadService.GetUnreadCount(g.ID); got != 1 {
		t.Fatalf("unread after 1 incoming = %d, want 1", got)
	}
}

func TestUnreadServiceMarkAsRead(t *testing.T) {
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	unreadService := NewUnreadService(store)

	g, _ := Create("Test Group", "host-1", Open)

	// Save 3 incoming messages
	for i := 0; i < 3; i++ {
		msg := GroupMessage{
			MessageID:  fmt.Sprintf("msg-markread-%d", i),
			GroupID:    g.ID,
			SenderID:   "member-1",
			SenderName: "Member One",
			Body:       fmt.Sprintf("Message %d", i),
			Timestamp:  time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		if err := adapter.SaveIncoming(g, msg); err != nil {
			t.Fatal(err)
		}
	}

	if got := unreadService.GetUnreadCount(g.ID); got != 3 {
		t.Fatalf("unread before MarkAsRead = %d, want 3", got)
	}

	if err := unreadService.MarkAsRead(g.ID); err != nil {
		t.Fatal(err)
	}

	if got := unreadService.GetUnreadCount(g.ID); got != 0 {
		t.Fatalf("unread after MarkAsRead = %d, want 0", got)
	}
}

func TestUnreadServiceSeparateGroups(t *testing.T) {
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	unreadService := NewUnreadService(store)

	gA, _ := Create("Group A", "host-a", Open)
	gB, _ := Create("Group B", "host-b", Open)

	// Same GroupCode, different GroupID
	gA.Code = "12345678"
	gB.Code = "12345678"

	// Incoming to Group A
	msgA := GroupMessage{
		MessageID:  "msg-unread-a-1",
		GroupID:    gA.ID,
		SenderID:   "member-a",
		SenderName: "Member A",
		Body:       "Hello A",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(gA, msgA); err != nil {
		t.Fatal(err)
	}

	// Incoming to Group B
	msgB := GroupMessage{
		MessageID:  "msg-unread-b-1",
		GroupID:    gB.ID,
		SenderID:   "member-b",
		SenderName: "Member B",
		Body:       "Hello B",
		Timestamp:  time.Now().Add(time.Millisecond),
	}
	if err := adapter.SaveIncoming(gB, msgB); err != nil {
		t.Fatal(err)
	}

	if got := unreadService.GetUnreadCount(gA.ID); got != 1 {
		t.Fatalf("Group A unread = %d, want 1", got)
	}
	if got := unreadService.GetUnreadCount(gB.ID); got != 1 {
		t.Fatalf("Group B unread = %d, want 1", got)
	}

	// Mark Group A as read
	if err := unreadService.MarkAsRead(gA.ID); err != nil {
		t.Fatal(err)
	}

	if got := unreadService.GetUnreadCount(gA.ID); got != 0 {
		t.Fatalf("Group A unread after mark = %d, want 0", got)
	}
	if got := unreadService.GetUnreadCount(gB.ID); got != 1 {
		t.Fatalf("Group B unread after mark A = %d, want 1", got)
	}
}

func TestUnreadServiceHasConversation(t *testing.T) {
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	unreadService := NewUnreadService(store)

	g, _ := Create("Test Group", "host-1", Open)

	if unreadService.HasConversation(g.ID) {
		t.Fatal("HasConversation() = true before any message")
	}

	msg := GroupMessage{
		MessageID:  "msg-hasconv-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	if !unreadService.HasConversation(g.ID) {
		t.Fatal("HasConversation() = false after message")
	}
}

func TestGetConversationID(t *testing.T) {
	convID := GetConversationID("group-123")
	expected := "group:group-123"
	if convID != expected {
		t.Fatalf("GetConversationID() = %q, want %q", convID, expected)
	}
}

func TestValidateGroupMessageForUnread(t *testing.T) {
	valid := GroupMessage{MessageID: "msg-1", GroupID: "group-1"}
	if err := ValidateGroupMessageForUnread(valid); err != nil {
		t.Fatalf("ValidateGroupMessageForUnread(valid) error = %v", err)
	}

	if err := ValidateGroupMessageForUnread(GroupMessage{GroupID: "group-1"}); err == nil {
		t.Fatal("ValidateGroupMessageForUnread() with empty MessageID succeeded")
	}

	if err := ValidateGroupMessageForUnread(GroupMessage{MessageID: "msg-1"}); err == nil {
		t.Fatal("ValidateGroupMessageForUnread() with empty GroupID succeeded")
	}
}
