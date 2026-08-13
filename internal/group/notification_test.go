package group

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockNotifier implements Notifier for testing.
type mockNotifier struct {
	mu       sync.Mutex
	notified []string
}

func (m *mockNotifier) SendNotification(title, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notified = append(m.notified, fmt.Sprintf("%s|%s", title, content))
}

func (m *mockNotifier) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.notified)
}

func (m *mockNotifier) Last() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.notified) == 0 {
		return ""
	}
	return m.notified[len(m.notified)-1]
}

func TestNotificationServiceIncoming(t *testing.T) {
	notifier := &mockNotifier{}
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	service := NewNotificationService(notifier, adapter)

	g, _ := Create("Test Group", "host-1", Open)
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}

	msg := GroupMessage{
		MessageID:  "msg-notif-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Hello notification",
		Timestamp:  time.Now(),
	}

	if err := service.NotifyIncoming(g, msg); err != nil {
		t.Fatalf("NotifyIncoming() error = %v", err)
	}
	if notifier.Count() != 1 {
		t.Fatalf("notification count = %d, want 1", notifier.Count())
	}
	expected := "Test Group|Member One: Hello notification"
	if notifier.Last() != expected {
		t.Fatalf("notification = %q, want %q", notifier.Last(), expected)
	}
}

func TestNotificationServiceDuplicateRejected(t *testing.T) {
	notifier := &mockNotifier{}
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	service := NewNotificationService(notifier, adapter)

	g, _ := Create("Test Group", "host-1", Open)
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}

	msg := GroupMessage{
		MessageID:  "msg-notif-dup-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}

	// Save first (simulating history persistence)
	if err := adapter.SaveIncoming(g, msg); err != nil {
		t.Fatal(err)
	}

	// Try to notify same message again
	if err := service.NotifyIncoming(g, msg); err == nil {
		t.Fatal("NotifyIncoming() duplicate succeeded")
	}
	if notifier.Count() != 0 {
		t.Fatalf("notification count = %d, want 0", notifier.Count())
	}
}

func TestNotificationServiceClosedGroupRejected(t *testing.T) {
	notifier := &mockNotifier{}
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	service := NewNotificationService(notifier, adapter)

	g, _ := Create("Test Group", "host-1", Open)
	g.Close()

	msg := GroupMessage{
		MessageID:  "msg-notif-closed-1",
		GroupID:    g.ID,
		SenderID:   "member-1",
		SenderName: "Member One",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}

	if err := service.NotifyIncoming(g, msg); err == nil {
		t.Fatal("NotifyIncoming() on closed group succeeded")
	}
	if notifier.Count() != 0 {
		t.Fatalf("notification count = %d, want 0", notifier.Count())
	}
}

func TestNotificationServiceNonMemberRejected(t *testing.T) {
	notifier := &mockNotifier{}
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	service := NewNotificationService(notifier, adapter)

	g, _ := Create("Test Group", "host-1", Open)

	msg := GroupMessage{
		MessageID:  "msg-notif-nonmember-1",
		GroupID:    g.ID,
		SenderID:   "non-member",
		SenderName: "Non Member",
		Body:       "Hello",
		Timestamp:  time.Now(),
	}

	if err := service.NotifyIncoming(g, msg); err == nil {
		t.Fatal("NotifyIncoming() for non-member succeeded")
	}
	if notifier.Count() != 0 {
		t.Fatalf("notification count = %d, want 0", notifier.Count())
	}
}

func TestNotificationServiceOutgoingNoNotification(t *testing.T) {
	notifier := &mockNotifier{}
	store, err := newTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHistoryAdapter(store)
	service := NewNotificationService(notifier, adapter)

	g, _ := Create("Test Group", "host-1", Open)
	msg := GroupMessage{
		MessageID:  "msg-notif-out-1",
		GroupID:    g.ID,
		SenderID:   "host-1",
		SenderName: "Host One",
		Body:       "Outgoing",
		Timestamp:  time.Now(),
	}

	if err := service.NotifyOutgoing(g, msg); err != nil {
		t.Fatal(err)
	}
	if notifier.Count() != 0 {
		t.Fatalf("notification count = %d, want 0 for outgoing", notifier.Count())
	}
}

func TestNotificationTruncation(t *testing.T) {
	longBody := ""
	for i := 0; i < 100; i++ {
		longBody += "a"
	}
	truncated := truncate(longBody, 80)
	if len(truncated) > 80 {
		t.Fatalf("truncated length = %d, want <= 80", len(truncated))
	}
	if len(truncated) < 77 {
		t.Fatalf("truncated length = %d, want >= 77", len(truncated))
	}
}
