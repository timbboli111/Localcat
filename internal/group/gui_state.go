package group

import (
	"fmt"
	"sort"
	"sync"
)

// GUIConversation represents a conversation entry for the GUI sidebar.
type GUIConversation struct {
	ID          string // conversation ID (direct:<peerID> or group:<groupID>)
	Type        string // "direct" or "group"
	Title       string
	UnreadCount int
	LastMessage string
	IsGroup     bool
	Closed      bool
}

// GUIState manages the UI-facing state for both direct and group conversations.
type GUIState struct {
	mu            sync.RWMutex
	conversations map[string]*GUIConversation // key: conversation ID
	order         []string                    // sorted by last activity
}

// NewGUIState creates an empty GUIState.
func NewGUIState() *GUIState {
	return &GUIState{
		conversations: make(map[string]*GUIConversation),
		order:         make([]string, 0),
	}
}

// UpsertGroup adds or updates a group conversation entry.
func (s *GUIState) UpsertGroup(g *Group, unread int, lastMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	convID := "group:" + g.ID
	conv, exists := s.conversations[convID]
	if !exists {
		conv = &GUIConversation{
			ID:      convID,
			Type:    "group",
			IsGroup: true,
		}
		s.conversations[convID] = conv
		s.order = append(s.order, convID)
	}
	conv.Title = g.Name
	conv.Closed = g.Closed
	conv.UnreadCount = unread
	conv.LastMessage = lastMsg
}

// UpsertDirect adds or updates a direct conversation entry.
func (s *GUIState) UpsertDirect(peerID, peerName string, unread int, lastMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	convID := "direct:" + peerID
	conv, exists := s.conversations[convID]
	if !exists {
		conv = &GUIConversation{
			ID:      convID,
			Type:    "direct",
			IsGroup: false,
		}
		s.conversations[convID] = conv
		s.order = append(s.order, convID)
	}
	conv.Title = peerName
	conv.UnreadCount = unread
	conv.LastMessage = lastMsg
}

// Remove removes a conversation entry.
func (s *GUIState) Remove(convID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conversations, convID)
	for i, id := range s.order {
		if id == convID {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// Get returns a conversation by ID.
func (s *GUIState) Get(convID string) (*GUIConversation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, exists := s.conversations[convID]
	return conv, exists
}

// All returns all conversations sorted alphabetically by title.
func (s *GUIState) All() []*GUIConversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*GUIConversation, 0, len(s.conversations))
	for _, conv := range s.conversations {
		out = append(out, conv)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Title < out[j].Title
	})
	return out
}

// MarkRead sets unread count to 0 for a conversation.
func (s *GUIState) MarkRead(convID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conv, exists := s.conversations[convID]; exists {
		conv.UnreadCount = 0
	}
}

// SetUnread sets the unread count for a conversation.
func (s *GUIState) SetUnread(convID string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conv, exists := s.conversations[convID]; exists {
		conv.UnreadCount = count
	}
}

// FormatGroupCode validates and formats a group code.
func FormatGroupCode(code string) string {
	var digits []rune
	for _, c := range code {
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	if len(digits) != 8 {
		return ""
	}
	return string(digits)
}

// ValidateGroupCodeInput checks if the input looks like a valid group code.
func ValidateGroupCodeInput(input string) error {
	formatted := FormatGroupCode(input)
	if formatted == "" {
		return fmt.Errorf("group code must be exactly 8 digits")
	}
	return nil
}
