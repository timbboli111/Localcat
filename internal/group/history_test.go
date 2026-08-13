package group

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"localcat/internal/history"
)

func TestGroupConversationCreated(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)
	msg := GroupMessage{
		MessageID:  "msg-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Hello group",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatalf("SaveIncoming() error = %v", err)
	}

	convID := history.GroupID(g.ID)
	msgs := store.Messages(convID)
	if len(msgs) != 1 {
		t.Fatalf("messages len = %d, want 1", len(msgs))
	}
	if msgs[0].MessageID != "msg-1" {
		t.Fatalf("MessageID = %q, want msg-1", msgs[0].MessageID)
	}
	if msgs[0].SenderName != "Member One" {
		t.Fatalf("SenderName = %q, want Member One", msgs[0].SenderName)
	}
	if msgs[0].Text != "Hello group" {
		t.Fatalf("Text = %q, want Hello group", msgs[0].Text)
	}
	if msgs[0].Outgoing {
		t.Fatal("Outgoing = true, want false for incoming")
	}
}

func TestGroupIDUsedAsConversationIdentity(t *testing.T) {
	convID := history.GroupID("group-abc-123")
	if convID != "group:group-abc-123" {
		t.Fatalf("GroupID() = %q, want group:group-abc-123", convID)
	}
}

func TestGroupMessagePersistenceAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)
	msg := GroupMessage{
		MessageID:  "msg-persist-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Persistent message",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	// Reopen store
	store2, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	msgs := store2.Messages(history.GroupID(g.ID))
	if len(msgs) != 1 {
		t.Fatalf("messages after reload = %d, want 1", len(msgs))
	}
	if msgs[0].MessageID != "msg-persist-1" {
		t.Fatalf("MessageID after reload = %q, want msg-persist-1", msgs[0].MessageID)
	}
	if msgs[0].Text != "Persistent message" {
		t.Fatalf("Text after reload = %q, want Persistent message", msgs[0].Text)
	}
}

func TestDuplicateMessageIDRejected(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)
	msg := GroupMessage{
		MessageID:  "msg-dup-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "First",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	// Try to save same MessageID again
	msg2 := msg
	msg2.Body = "Duplicate"
	if err := adapter.SaveIncoming(g, msg2); err == nil {
		t.Fatal("SaveIncoming() with duplicate MessageID succeeded")
	}

	msgs := store.Messages(history.GroupID(g.ID))
	if len(msgs) != 1 {
		t.Fatalf("messages after duplicate = %d, want 1", len(msgs))
	}
}

func TestIncomingGroupMessageIncrementsUnread(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)

	// First incoming
	msg1 := GroupMessage{
		MessageID:  "msg-unread-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "First",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg1); err != nil {
		t.Fatal(err)
	}

	// Second incoming
	msg2 := GroupMessage{
		MessageID:  "msg-unread-2",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Second",
		Timestamp:  time.Now().Add(time.Millisecond),
	}
	if err := adapter.SaveIncoming(g, msg2); err != nil {
		t.Fatal(err)
	}

	conversations := store.Conversations()
	if len(conversations) != 1 {
		t.Fatalf("conversations = %d, want 1", len(conversations))
	}
	if conversations[0].UnreadCount != 2 {
		t.Fatalf("UnreadCount = %d, want 2", conversations[0].UnreadCount)
	}
}

func TestOutgoingGroupMessageDoesNotIncrementUnread(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)

	// Incoming
	msgIn := GroupMessage{
		MessageID:  "msg-in-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Incoming",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msgIn); err != nil {
		t.Fatal(err)
	}

	// Outgoing
	msgOut := GroupMessage{
		MessageID:  "msg-out-1",
		GroupID:    g.ID,
		SenderID:   "host-1",
		SenderName: "Host One",
		Body:       "Outgoing",
		Timestamp:  time.Now().Add(time.Millisecond),
	}
	if err := adapter.SaveOutgoing(g, msgOut); err != nil {
		t.Fatal(err)
	}

	conversations := store.Conversations()
	if len(conversations) != 1 {
		t.Fatalf("conversations = %d, want 1", len(conversations))
	}
	if conversations[0].UnreadCount != 1 {
		t.Fatalf("UnreadCount after outgoing = %d, want 1", conversations[0].UnreadCount)
	}
}

func TestMarkAsReadResetsGroupUnread(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)

	for i := 0; i < 3; i++ {
		msg := GroupMessage{
			MessageID:  fmt.Sprintf("msg-read-%d", i),
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

	convID := history.GroupID(g.ID)
	if err := store.MarkAsRead(convID); err != nil {
		t.Fatal(err)
	}

	conversations := store.Conversations()
	if len(conversations) != 1 {
		t.Fatalf("conversations = %d, want 1", len(conversations))
	}
	if conversations[0].UnreadCount != 0 {
		t.Fatalf("UnreadCount after MarkAsRead = %d, want 0", conversations[0].UnreadCount)
	}
}

func TestSenderNameSnapshotStored(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)
	msg := GroupMessage{
		MessageID:  "msg-snapshot-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Original Name",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	msgs := store.Messages(history.GroupID(g.ID))
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	if msgs[0].SenderName != "Original Name" {
		t.Fatalf("SenderName = %q, want Original Name", msgs[0].SenderName)
	}
}

func TestTwoGroupsSameCodeDifferentHistory(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	gA, _ := Create("Group A", "host-a", Open)
	gA.Code = "12345678"

	gB, _ := Create("Group B", "host-b", Open)
	gB.Code = "12345678"

	msgA := GroupMessage{
		MessageID:  "msg-a-1",
		GroupID:    gA.ID,
		SenderID:   "member-a",
		SenderName: "Member A",
		Body:       "Message A",
		Timestamp:  time.Now(),
	}
	msgB := GroupMessage{
		MessageID:  "msg-b-1",
		GroupID:    gB.ID,
		SenderID:   "member-b",
		SenderName: "Member B",
		Body:       "Message B",
		Timestamp:  time.Now().Add(time.Millisecond),
	}

	if err := adapter.SaveIncoming(gA, msgA); err != nil {
		t.Fatal(err)
	}
	if err := adapter.SaveIncoming(gB, msgB); err != nil {
		t.Fatal(err)
	}

	msgsA := store.Messages(history.GroupID(gA.ID))
	msgsB := store.Messages(history.GroupID(gB.ID))

	if len(msgsA) != 1 || msgsA[0].Text != "Message A" {
		t.Fatalf("Group A messages wrong: %+v", msgsA)
	}
	if len(msgsB) != 1 || msgsB[0].Text != "Message B" {
		t.Fatalf("Group B messages wrong: %+v", msgsB)
	}
	if history.GroupID(gA.ID) == history.GroupID(gB.ID) {
		t.Fatal("Group A and B have same conversation ID")
	}
}

func TestClosedGroupHistoryStillReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)

	g, _ := Create("Test Group", "host-1", Open)
	msg := GroupMessage{
		MessageID:  "msg-closed-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Before close",
		Timestamp:  time.Now(),
	}
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	g.Close()

	store2, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	msgs := store2.Messages(history.GroupID(g.ID))
	if len(msgs) != 1 {
		t.Fatalf("messages after close+reload = %d, want 1", len(msgs))
	}
}

func TestBackwardCompatibleHistoryFile(t *testing.T) {
	oldJSON := `{
		"conversations": {
			"direct:peer-1": {
				"id": "direct:peer-1",
				"type": "direct",
				"title": "peer-1",
				"participant": "peer-1",
				"updated_at": "2026-08-14T10:00:00Z",
				"unread_count": 1
			}
		},
		"messages": [
			{
				"id": "123456789",
				"conversation_id": "direct:peer-1",
				"sender_id": "peer-1",
				"sender_name": "Peer One",
				"text": "Old message",
				"sent_at": "2026-08-14T10:00:00Z",
				"outgoing": false
			}
		]
	}`
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte(oldJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := history.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	msgs := store.Messages(history.DirectID("peer-1"))
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	if msgs[0].MessageID != "" {
		t.Fatalf("MessageID = %q, want empty for old messages", msgs[0].MessageID)
	}
	if msgs[0].Text != "Old message" {
		t.Fatalf("Text = %q, want Old message", msgs[0].Text)
	}
}
