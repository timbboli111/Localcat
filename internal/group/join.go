package group

import (
	"fmt"
	"strings"
)

// JoinResult describes the outcome of a join attempt.
type JoinResult struct {
	Status    string // "MEMBER", "PENDING", or "REJECTED"
	GroupID   string
	GroupName string
	GroupCode string
	HostID    string
	Reason    string
}

// JoinOpenGroup attempts to join an OPEN group.
// The caller must be the Host-side manager that owns the group.
// In a real network scenario, this would be invoked on the Host after
// receiving a join request.
func JoinOpenGroup(m *Manager, groupID, requesterID, requesterName string) (JoinResult, error) {
	requesterID = strings.TrimSpace(requesterID)
	requesterName = strings.TrimSpace(requesterName)
	if requesterID == "" {
		return JoinResult{}, fmt.Errorf("requester id is required")
	}
	if requesterName == "" {
		return JoinResult{}, fmt.Errorf("requester name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	g, exists := m.groups[groupID]
	if !exists {
		return JoinResult{}, ErrGroupNotFound
	}
	if g.Closed {
		return JoinResult{}, fmt.Errorf("group is closed")
	}
	if g.JoinPolicy != Open {
		return JoinResult{}, fmt.Errorf("group is not open")
	}
	if g.HasMember(requesterID) {
		return JoinResult{}, fmt.Errorf("requester %q is already a member", requesterID)
	}
	if err := g.AddMember(requesterID, requesterName); err != nil {
		return JoinResult{}, err
	}
	return JoinResult{
		Status:    "MEMBER",
		GroupID:   g.ID,
		GroupName: g.Name,
		GroupCode: g.Code,
		HostID:    g.HostID,
	}, nil
}

// RequestJoinLockedGroup creates a PENDING join request for a LOCKED group.
func RequestJoinLockedGroup(m *Manager, groupID, requesterID, requesterName string) (JoinResult, error) {
	requesterID = strings.TrimSpace(requesterID)
	requesterName = strings.TrimSpace(requesterName)
	if requesterID == "" {
		return JoinResult{}, fmt.Errorf("requester id is required")
	}
	if requesterName == "" {
		return JoinResult{}, fmt.Errorf("requester name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	g, exists := m.groups[groupID]
	if !exists {
		return JoinResult{}, ErrGroupNotFound
	}
	if g.Closed {
		return JoinResult{}, fmt.Errorf("group is closed")
	}
	if g.JoinPolicy != Locked {
		return JoinResult{}, fmt.Errorf("group is not locked")
	}
	if g.HasMember(requesterID) {
		return JoinResult{}, fmt.Errorf("requester %q is already a member", requesterID)
	}

	pr, exists := m.requests[groupID]
	if !exists {
		return JoinResult{}, fmt.Errorf("pending requests not initialized for group %q", groupID)
	}

	req := JoinRequest{
		GroupID:       groupID,
		RequesterID:   requesterID,
		RequesterName: requesterName,
		Status:        Pending,
		Timestamp:     TimeNow(),
	}
	if err := pr.Add(req, false); err != nil {
		return JoinResult{}, err
	}
	return JoinResult{
		Status:    "PENDING",
		GroupID:   g.ID,
		GroupName: g.Name,
		GroupCode: g.Code,
		HostID:    g.HostID,
	}, nil
}
