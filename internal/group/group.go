package group

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// JoinPolicy determines how new members can join a group.
type JoinPolicy int

const (
	// Open allows anyone with the GroupCode to join immediately.
	Open JoinPolicy = iota
	// Locked requires the Host to approve each join request.
	Locked
)

func (p JoinPolicy) String() string {
	switch p {
	case Open:
		return "OPEN"
	case Locked:
		return "LOCKED"
	default:
		return "UNKNOWN"
	}
}

// Role describes a member's authority within a group.
type Role int

const (
	// RoleMember is a regular participant with no administrative authority.
	RoleMember Role = iota
	// RoleHost is the single owner and authority of the group.
	RoleHost
)

func (r Role) String() string {
	switch r {
	case RoleHost:
		return "HOST"
	case RoleMember:
		return "MEMBER"
	default:
		return "UNKNOWN"
	}
}

// RequestStatus tracks the lifecycle of a join request.
type RequestStatus int

const (
	// Pending means the request is awaiting Host decision.
	Pending RequestStatus = iota
	// Accepted means the Host approved the request.
	Accepted
	// Rejected means the Host declined the request.
	Rejected
)

func (s RequestStatus) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Accepted:
		return "ACCEPTED"
	case Rejected:
		return "REJECTED"
	default:
		return "UNKNOWN"
	}
}

// Group is a chat room owned by a single Host.
type Group struct {
	ID         string
	Code       string
	Name       string
	HostID     string
	JoinPolicy JoinPolicy
	Members    map[string]Member
	CreatedAt  time.Time
	Closed     bool
}

// Member is a participant in a group.
type Member struct {
	ID       string
	Name     string
	Role     Role
	JoinedAt time.Time
}

// JoinRequest is a request to join a LOCKED group.
type JoinRequest struct {
	GroupID       string
	RequesterID   string
	RequesterName string
	Status        RequestStatus
	Timestamp     time.Time
}

var groupCodePattern = regexp.MustCompile(`^[0-9]{8}$`)

// NewID generates a 128-bit random GroupID as 32 hex characters.
func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate group id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// NewCode generates a user-facing 8-digit numeric GroupCode.
// The code may include leading zeros. It is not a password or authority.
func NewCode() (string, error) {
	for i := 0; i < 5; i++ {
		var n uint32
		if err := randomUint32(&n); err != nil {
			return "", err
		}
		code := fmt.Sprintf("%08d", n%100000000)
		if groupCodePattern.MatchString(code) {
			return code, nil
		}
	}
	return "", fmt.Errorf("generate group code: failed after retries")
}

func randomUint32(out *uint32) error {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	*out = uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	return nil
}

// Create builds a new Group with a random ID and Code.
// The Host is added as the first member with Role RoleHost.
func Create(name, hostID string, policy JoinPolicy) (*Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("group name is required")
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return nil, fmt.Errorf("host id is required")
	}

	id, err := NewID()
	if err != nil {
		return nil, err
	}
	code, err := NewCode()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	g := &Group{
		ID:         id,
		Code:       code,
		Name:       name,
		HostID:     hostID,
		JoinPolicy: policy,
		Members:    make(map[string]Member),
		CreatedAt:  now,
		Closed:     false,
	}
	g.Members[hostID] = Member{
		ID:       hostID,
		Name:     hostID, // placeholder; caller may update
		Role:     RoleHost,
		JoinedAt: now,
	}
	return g, nil
}

// SetHostDisplayName updates the Host member's cached display name.
func (g *Group) SetHostDisplayName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("display name is required")
	}
	host, exists := g.Members[g.HostID]
	if !exists {
		return fmt.Errorf("host member not found for id %q", g.HostID)
	}
	host.Name = name
	g.Members[g.HostID] = host
	return nil
}

// Validate checks that the Group is structurally valid.
func (g *Group) Validate() error {
	if g.ID == "" {
		return fmt.Errorf("group id is required")
	}
	if !groupCodePattern.MatchString(g.Code) {
		return fmt.Errorf("group code must be exactly 8 digits")
	}
	if strings.TrimSpace(g.Name) == "" {
		return fmt.Errorf("group name is required")
	}
	if g.HostID == "" {
		return fmt.Errorf("host id is required")
	}
	if _, exists := g.Members[g.HostID]; !exists {
		return fmt.Errorf("host must be a member of the group")
	}

	hostCount := 0
	for id, m := range g.Members {
		if id == "" {
			return fmt.Errorf("member id cannot be empty")
		}
		if m.ID == "" {
			return fmt.Errorf("member struct id cannot be empty")
		}
		if m.ID != id {
			return fmt.Errorf("member map key %q does not match member id %q", id, m.ID)
		}
		if m.Role == RoleHost {
			hostCount++
		}
		if m.Role != RoleHost && m.Role != RoleMember {
			return fmt.Errorf("member %q has invalid role", id)
		}
	}
	if hostCount != 1 {
		return fmt.Errorf("group must have exactly one host, got %d", hostCount)
	}
	if g.Members[g.HostID].Role != RoleHost {
		return fmt.Errorf("group host must have role RoleHost")
	}
	return nil
}

// AddMember adds a new member to the group.
func (g *Group) AddMember(id, name string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("member id is required")
	}
	if g.Closed {
		return fmt.Errorf("group is closed")
	}
	if _, exists := g.Members[id]; exists {
		return fmt.Errorf("member %q already exists", id)
	}
	g.Members[id] = Member{
		ID:       id,
		Name:     strings.TrimSpace(name),
		Role:     RoleMember,
		JoinedAt: time.Now(),
	}
	return nil
}

// RemoveMember removes a non-host member from the group.
func (g *Group) RemoveMember(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("member id is required")
	}
	if id == g.HostID {
		return fmt.Errorf("host cannot be removed through RemoveMember")
	}
	m, exists := g.Members[id]
	if !exists {
		return fmt.Errorf("member %q not found", id)
	}
	if m.Role == RoleHost {
		return fmt.Errorf("host cannot be removed through RemoveMember")
	}
	delete(g.Members, id)
	return nil
}

// HasMember reports whether the identity ID belongs to the group.
func (g *Group) HasMember(id string) bool {
	_, exists := g.Members[id]
	return exists
}

// GetMember returns a member by identity ID.
func (g *Group) GetMember(id string) (Member, bool) {
	m, exists := g.Members[id]
	return m, exists
}

// IsHost reports whether the identity ID is the group Host.
func (g *Group) IsHost(id string) bool {
	return id == g.HostID
}

// Close permanently closes the group.
func (g *Group) Close() {
	g.Closed = true
}

// PendingRequests tracks join requests for a specific group.
type PendingRequests struct {
	requests map[string]JoinRequest // key: RequesterID
}

// NewPendingRequests creates an empty PendingRequests collection.
func NewPendingRequests() *PendingRequests {
	return &PendingRequests{requests: make(map[string]JoinRequest)}
}

// Add adds a pending join request.
// It returns an error if the requester already has a pending request,
// if the requester is already a member, or if the status is not Pending.
func (pr *PendingRequests) Add(req JoinRequest, isMember bool) error {
	req.RequesterID = strings.TrimSpace(req.RequesterID)
	if req.RequesterID == "" {
		return fmt.Errorf("requester id is required")
	}
	if isMember {
		return fmt.Errorf("requester %q is already a member", req.RequesterID)
	}
	if req.Status != Pending {
		return fmt.Errorf("Add only accepts Pending requests")
	}
	if _, exists := pr.requests[req.RequesterID]; exists {
		return fmt.Errorf("pending request for requester %q already exists", req.RequesterID)
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}
	pr.requests[req.RequesterID] = req
	return nil
}

// GetPending returns the pending request for a requester, if any.
func (pr *PendingRequests) GetPending(requesterID string) (JoinRequest, bool) {
	req, exists := pr.requests[requesterID]
	if !exists || req.Status != Pending {
		return JoinRequest{}, false
	}
	return req, true
}

// Remove removes a request by requester ID.
func (pr *PendingRequests) Remove(requesterID string) {
	delete(pr.requests, requesterID)
}

// Count returns the number of pending requests.
func (pr *PendingRequests) Count() int {
	return len(pr.requests)
}

// All returns all pending join requests.
func (pr *PendingRequests) All() []JoinRequest {
	out := make([]JoinRequest, 0, len(pr.requests))
	for _, req := range pr.requests {
		out = append(out, req)
	}
	return out
}
