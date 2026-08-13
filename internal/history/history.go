package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const DirectConversation = "direct"

type Conversation struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Participant string    `json:"participant,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	UnreadCount int       `json:"unread_count,omitempty"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Text           string    `json:"text"`
	SentAt         time.Time `json:"sent_at"`
	Outgoing       bool      `json:"outgoing"`
}

type fileData struct {
	Conversations map[string]Conversation `json:"conversations"`
	Messages      []Message               `json:"messages"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data fileData
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, data: fileData{Conversations: map[string]Conversation{}}}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read history: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}
	if s.data.Conversations == nil {
		s.data.Conversations = map[string]Conversation{}
	}
	return s, nil
}

func DirectID(peerID string) string { return DirectConversation + ":" + peerID }

func (s *Store) AddMessage(conv Conversation, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conv.ID == "" || conv.Type == "" {
		return fmt.Errorf("conversation id and type are required")
	}
	if msg.Text = strings.TrimSpace(msg.Text); msg.Text == "" {
		return fmt.Errorf("message text is required")
	}
	if msg.SentAt.IsZero() {
		msg.SentAt = time.Now()
	}
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("%d", msg.SentAt.UnixNano())
	}
	msg.ConversationID = conv.ID

	// Preserve existing unread count if conversation already exists.
	// Callers may pass a fresh Conversation with UnreadCount 0, which
	// would otherwise reset the counter before incrementing.
	if existingConv, exists := s.data.Conversations[conv.ID]; exists {
		conv.UnreadCount = existingConv.UnreadCount
	}

	conv.UpdatedAt = msg.SentAt
	if !msg.Outgoing {
		conv.UnreadCount++
	}
	s.data.Conversations[conv.ID] = conv
	s.data.Messages = append(s.data.Messages, msg)
	return s.saveLocked()
}

func (s *Store) Messages(conversationID string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Message
	for _, m := range s.data.Messages {
		if m.ConversationID == conversationID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SentAt.Before(out[j].SentAt) })
	return out
}

func (s *Store) Conversations() []Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Conversation, 0, len(s.data.Conversations))
	for _, c := range s.data.Conversations {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (s *Store) MarkAsRead(conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conv, exists := s.data.Conversations[conversationID]
	if !exists || conv.UnreadCount == 0 {
		return nil
	}
	conv.UnreadCount = 0
	s.data.Conversations[conversationID] = conv
	return s.saveLocked()
}

func (s *Store) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Conversations, id)
	filtered := s.data.Messages[:0]
	for _, m := range s.data.Messages {
		if m.ConversationID != id {
			filtered = append(filtered, m)
		}
	}
	s.data.Messages = filtered
	return s.saveLocked()
}

func (s *Store) DeleteAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = fileData{Conversations: map[string]Conversation{}, Messages: nil}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode history: %w", err)
	}
	return os.WriteFile(s.path, b, 0o600)
}
