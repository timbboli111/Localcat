package group

import (
	"fmt"
	"sync"
)

// Relay handles Group Message distribution from Host to Members.
// It is safe for concurrent use.
type Relay struct {
	mu     sync.RWMutex
	groups map[string]*RelayGroup // key: GroupID
}

// RelayGroup holds the state needed to relay messages for one group.
type RelayGroup struct {
	group       *Group
	memberPeers map[string]RelayPeer // key: MemberID
}

// RelayPeer describes where to send a relayed message.
type RelayPeer struct {
	Address string
	Port    int
}

// NewRelay creates an empty Relay.
func NewRelay() *Relay {
	return &Relay{groups: make(map[string]*RelayGroup)}
}

// RegisterGroup registers a group for relay.
func (r *Relay) RegisterGroup(g *Group) error {
	if g == nil {
		return fmt.Errorf("group is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.groups[g.ID]; exists {
		return fmt.Errorf("group %q already registered", g.ID)
	}
	r.groups[g.ID] = &RelayGroup{
		group:       g,
		memberPeers: make(map[string]RelayPeer),
	}
	return nil
}

// UnregisterGroup removes a group from relay.
func (r *Relay) UnregisterGroup(groupID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.groups, groupID)
}

// SetMemberPeer associates a member with their network address.
func (r *Relay) SetMemberPeer(groupID, memberID, address string, port int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rg, exists := r.groups[groupID]
	if !exists {
		return fmt.Errorf("group %q not registered", groupID)
	}
	if !rg.group.HasMember(memberID) {
		return fmt.Errorf("member %q not in group %q", memberID, groupID)
	}
	rg.memberPeers[memberID] = RelayPeer{Address: address, Port: port}
	return nil
}

// RemoveMemberPeer removes a member's network address.
func (r *Relay) RemoveMemberPeer(groupID, memberID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rg, exists := r.groups[groupID]; exists {
		delete(rg.memberPeers, memberID)
	}
}

// GetRelayTargets returns the network addresses of all members except
// the given senderID. Returns an error if the group is unknown or
// the sender is not a member.
func (r *Relay) GetRelayTargets(groupID, senderID string) ([]RelayPeer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rg, exists := r.groups[groupID]
	if !exists {
		return nil, fmt.Errorf("group %q not registered", groupID)
	}
	if !rg.group.HasMember(senderID) {
		return nil, fmt.Errorf("sender %q not a member of group %q", senderID, groupID)
	}
	var targets []RelayPeer
	for memberID, peer := range rg.memberPeers {
		if memberID == senderID {
			continue // don't relay back to sender
		}
		targets = append(targets, peer)
	}
	return targets, nil
}

// ValidateMemberCanSend checks whether the sender is allowed to send
// a group message to the given group.
func (r *Relay) ValidateMemberCanSend(groupID, senderID string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rg, exists := r.groups[groupID]
	if !exists {
		return fmt.Errorf("group %q not registered", groupID)
	}
	if rg.group.Closed {
		return fmt.Errorf("group %q is closed", groupID)
	}
	if !rg.group.HasMember(senderID) {
		return fmt.Errorf("sender %q not a member of group %q", senderID, groupID)
	}
	return nil
}
