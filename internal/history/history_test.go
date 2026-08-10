package history

import "testing"

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
