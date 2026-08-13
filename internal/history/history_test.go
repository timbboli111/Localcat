package history

import (
	"fmt"
	"testing"
	"time"
)

func TestConversationSpecificDeletion(t *testing.T) {
	store, err := Open(t.TempDir() + "/history.json")
	if err != nil {
		t.Fatal(err)
	}
	add := func(peer, text string) {
		t.Helper()
		conv := Conversation{ID: DirectID(peer), Type: DirectConversation, Title: peer, Participant: peer}
		if err := store.AddMessage(conv, Message{SenderID: peer, SenderName: peer, Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	add("user-a", "a1")
	add("user-b", "b1")
	if err := store.DeleteConversation(DirectID("user-a")); err != nil {
		t.Fatal(err)
	}
	if got := len(store.Messages(DirectID("user-a"))); got != 0 {
		t.Fatalf("deleted conversation has %d messages", got)
	}
	if got := len(store.Messages(DirectID("user-b"))); got != 1 {
		t.Fatalf("other conversation has %d messages", got)
	}
}

func TestDeleteAllHistory(t *testing.T) {
	store, err := Open(t.TempDir() + "/history.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.AddMessage(Conversation{ID: DirectID("a"), Type: DirectConversation, Title: "a", Participant: "a"}, Message{SenderID: "a", SenderName: "a", Text: "hello"})
	if err := store.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	if got := len(store.Conversations()); got != 0 {
		t.Fatalf("conversations = %d", got)
	}
	if got := len(store.Messages(DirectID("a"))); got != 0 {
		t.Fatalf("messages = %d", got)
	}
}

func TestLocalDeletionDoesNotAffectAnotherStore(t *testing.T) {
	storeA, err := Open(t.TempDir() + "/a.json")
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := Open(t.TempDir() + "/b.json")
	if err != nil {
		t.Fatal(err)
	}
	conv := Conversation{ID: DirectID("peer"), Type: DirectConversation, Title: "peer", Participant: "peer"}
	_ = storeA.AddMessage(conv, Message{SenderID: "peer", SenderName: "peer", Text: "on a"})
	_ = storeB.AddMessage(conv, Message{SenderID: "peer", SenderName: "peer", Text: "on b"})
	if err := storeA.DeleteConversation(conv.ID); err != nil {
		t.Fatal(err)
	}
	if got := len(storeA.Messages(conv.ID)); got != 0 {
		t.Fatalf("store A messages = %d", got)
	}
	if got := len(storeB.Messages(conv.ID)); got != 1 {
		t.Fatalf("store B messages = %d", got)
	}
}

func TestUnreadCountIncrementsAcrossIncomingMessages(t *testing.T) {
	store, err := Open(t.TempDir() + "/history.json")
	if err != nil {
		t.Fatal(err)
	}
	convID := DirectID("peer-a")

	// TEST 1: First incoming message -> UnreadCount = 1
	conv1 := Conversation{ID: convID, Type: DirectConversation, Title: "peer-a", Participant: "peer-a"}
	if err := store.AddMessage(conv1, Message{SenderID: "peer-a", SenderName: "peer-a", Text: "first", Outgoing: false}); err != nil {
		t.Fatal(err)
	}
	if got := store.Conversations()[0].UnreadCount; got != 1 {
		t.Fatalf("after first incoming: unread = %d, want 1", got)
	}

	// TEST 2: Second incoming message with fresh Conversation object -> UnreadCount = 2
	conv2 := Conversation{ID: convID, Type: DirectConversation, Title: "peer-a", Participant: "peer-a"}
	if err := store.AddMessage(conv2, Message{SenderID: "peer-a", SenderName: "peer-a", Text: "second", Outgoing: false}); err != nil {
		t.Fatal(err)
	}
	if got := store.Conversations()[0].UnreadCount; got != 2 {
		t.Fatalf("after second incoming: unread = %d, want 2", got)
	}

	// TEST 3: Third incoming message -> UnreadCount = 3
	conv3 := Conversation{ID: convID, Type: DirectConversation, Title: "peer-a", Participant: "peer-a"}
	if err := store.AddMessage(conv3, Message{SenderID: "peer-a", SenderName: "peer-a", Text: "third", Outgoing: false}); err != nil {
		t.Fatal(err)
	}
	if got := store.Conversations()[0].UnreadCount; got != 3 {
		t.Fatalf("after third incoming: unread = %d, want 3", got)
	}

	// TEST 4: Outgoing message does not increment unread -> UnreadCount remains 3
	conv4 := Conversation{ID: convID, Type: DirectConversation, Title: "peer-a", Participant: "peer-a"}
	if err := store.AddMessage(conv4, Message{SenderID: "me", SenderName: "me", Text: "reply", Outgoing: true}); err != nil {
		t.Fatal(err)
	}
	if got := store.Conversations()[0].UnreadCount; got != 3 {
		t.Fatalf("after outgoing: unread = %d, want 3", got)
	}

	// TEST 5: MarkAsRead resets unread -> UnreadCount = 0
	if err := store.MarkAsRead(convID); err != nil {
		t.Fatal(err)
	}
	if got := store.Conversations()[0].UnreadCount; got != 0 {
		t.Fatalf("after MarkAsRead: unread = %d, want 0", got)
	}
}

func TestUnreadCountDoesNotAffectOtherConversations(t *testing.T) {
	store, err := Open(t.TempDir() + "/history.json")
	if err != nil {
		t.Fatal(err)
	}
	convA := DirectID("peer-a")
	convB := DirectID("peer-b")

	// Add incoming messages to conversation A
	for i := 0; i < 3; i++ {
		conv := Conversation{ID: convA, Type: DirectConversation, Title: "peer-a", Participant: "peer-a"}
		if err := store.AddMessage(conv, Message{
			SenderID:   "peer-a",
			SenderName: "peer-a",
			Text:       fmt.Sprintf("a-%d", i),
			SentAt:     time.Now().Add(time.Duration(i) * time.Millisecond),
			Outgoing:   false,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Add one incoming message to conversation B
	convB1 := Conversation{ID: convB, Type: DirectConversation, Title: "peer-b", Participant: "peer-b"}
	if err := store.AddMessage(convB1, Message{SenderID: "peer-b", SenderName: "peer-b", Text: "b-0", Outgoing: false}); err != nil {
		t.Fatal(err)
	}

	conversations := store.Conversations()
	if len(conversations) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(conversations))
	}

	// Find each conversation by ID since Conversations() returns sorted by UpdatedAt
	var unreadA, unreadB int
	for _, c := range conversations {
		switch c.ID {
		case convA:
			unreadA = c.UnreadCount
		case convB:
			unreadB = c.UnreadCount
		}
	}

	if unreadA != 3 {
		t.Fatalf("conversation A unread = %d, want 3", unreadA)
	}
	if unreadB != 1 {
		t.Fatalf("conversation B unread = %d, want 1", unreadB)
	}
}
