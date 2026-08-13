package group

import (
	"fmt"
	"sync"
	"time"
)

// Manager tracks groups where this peer is either Host or Member.
// It is safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	groups   map[string]*Group           // key: GroupID
	requests map[string]*PendingRequests // key: GroupID
}

// NewManager creates an empty group Manager.
func NewManager() *Manager {
	return &Manager{
		groups:   make(map[string]*Group),
		requests: make(map[string]*PendingRequests),
	}
}

// AddGroup registers a group in the manager.
func (m *Manager) AddGroup(g *Group) error {
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	if err := g.Validate(); err != nil {
		return fmt.Errorf("validate group: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.groups[g.ID]; exists {
		return fmt.Errorf("group %q already exists", g.ID)
	}
	m.groups[g.ID] = g
	m.requests[g.ID] = NewPendingRequests()
	return nil
}

// RemoveGroup removes a group and its pending requests.
func (m *Manager) RemoveGroup(groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.groups, groupID)
	delete(m.requests, groupID)
}

// GetGroup returns a group by ID.
func (m *Manager) GetGroup(groupID string) (*Group, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, exists := m.groups[groupID]
	return g, exists
}

// GetGroupByCode returns a group by its 8-digit code.
// If multiple groups share the same code, returns ErrGroupCodeAmbiguous.
func (m *Manager) GetGroupByCode(code string) (*Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var found *Group
	for _, g := range m.groups {
		if g.Code == code {
			if found != nil && found.ID != g.ID {
				return nil, ErrGroupCodeAmbiguous
			}
			found = g
		}
	}
	if found == nil {
		return nil, ErrGroupNotFound
	}
	return found, nil
}

// AllGroups returns a snapshot of all managed groups.
func (m *Manager) AllGroups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		out = append(out, g)
	}
	return out
}

// GetPendingRequests returns the pending request collection for a group.
func (m *Manager) GetPendingRequests(groupID string) (*PendingRequests, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pr, exists := m.requests[groupID]
	return pr, exists
}

// AddJoinRequest adds a pending join request for a group.
// It validates that the group exists, is not closed, the requester is
// not already a member, and the request is not a duplicate.
func (m *Manager) AddJoinRequest(groupID string, req JoinRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, exists := m.groups[groupID]
	if !exists {
		return ErrGroupNotFound
	}
	if g.Closed {
		return fmt.Errorf("group is closed")
	}
	if g.HasMember(req.RequesterID) {
		return fmt.Errorf("requester %q is already a member", req.RequesterID)
	}
	pr, exists := m.requests[groupID]
	if !exists {
		return fmt.Errorf("pending requests not initialized for group %q", groupID)
	}
	return pr.Add(req, false)
}

// AcceptJoinRequest accepts a pending join request.
// Only the Host can accept. The requester is added as a Member.
func (m *Manager) AcceptJoinRequest(groupID, requesterID, actingHostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, exists := m.groups[groupID]
	if !exists {
		return ErrGroupNotFound
	}
	if g.Closed {
		return fmt.Errorf("group is closed")
	}
	if actingHostID != g.HostID {
		return fmt.Errorf("only the host can accept join requests")
	}
	pr, exists := m.requests[groupID]
	if !exists {
		return fmt.Errorf("pending requests not initialized")
	}
	pending, exists := pr.GetPending(requesterID)
	if !exists {
		return fmt.Errorf("no pending request for requester %q", requesterID)
	}
	if pending.Status != Pending {
		return fmt.Errorf("request for %q is not pending", requesterID)
	}

	// Add as member
	if err := g.AddMember(requesterID, pending.RequesterName); err != nil {
		return err
	}
	pr.Remove(requesterID)
	return nil
}

// RejectJoinRequest rejects a pending join request.
// Only the Host can reject.
func (m *Manager) RejectJoinRequest(groupID, requesterID, actingHostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, exists := m.groups[groupID]
	if !exists {
		return ErrGroupNotFound
	}
	if actingHostID != g.HostID {
		return fmt.Errorf("only the host can reject join requests")
	}
	pr, exists := m.requests[groupID]
	if !exists {
		return fmt.Errorf("pending requests not initialized")
	}
	if _, exists := pr.GetPending(requesterID); !exists {
		return fmt.Errorf("no pending request for requester %q", requesterID)
	}
	pr.Remove(requesterID)
	return nil
}

// IsHost checks if the given identity ID is the host of the given group.
func (m *Manager) IsHost(groupID, identityID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, exists := m.groups[groupID]
	if !exists {
		return false
	}
	return g.IsHost(identityID)
}

// CloseGroup closes a group. Only the Host can close.
func (m *Manager) CloseGroup(groupID, actingHostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, exists := m.groups[groupID]
	if !exists {
		return ErrGroupNotFound
	}
	if actingHostID != g.HostID {
		return fmt.Errorf("only the host can close the group")
	}
	g.Close()
	return nil
}

// TimeNow is a variable for testability.
var TimeNow = time.Now
