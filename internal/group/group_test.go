package group

import (
	"regexp"
	"strings"
	"testing"
)

func TestCreateGroup(t *testing.T) {
	g, err := Create("Division", "host-identity", Locked)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if g.ID == "" {
		t.Fatal("Group.ID is empty")
	}
	if g.Code == "" {
		t.Fatal("Group.Code is empty")
	}
	if g.Name != "Division" {
		t.Fatalf("Group.Name = %q, want %q", g.Name, "Division")
	}
	if g.HostID != "host-identity" {
		t.Fatalf("Group.HostID = %q, want %q", g.HostID, "host-identity")
	}
	if g.JoinPolicy != Locked {
		t.Fatalf("Group.JoinPolicy = %v, want %v", g.JoinPolicy, Locked)
	}
	if g.Closed {
		t.Fatal("new group should not be closed")
	}
	if len(g.Members) != 1 {
		t.Fatalf("new group should have 1 member, got %d", len(g.Members))
	}
	host, exists := g.Members[g.HostID]
	if !exists {
		t.Fatal("host not found in members")
	}
	if host.Role != RoleHost {
		t.Fatalf("host role = %v, want %v", host.Role, RoleHost)
	}
	if host.ID != g.HostID {
		t.Fatalf("host member ID = %q, want %q", host.ID, g.HostID)
	}
}

func TestGroupIDEntropy(t *testing.T) {
	id1, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	id2, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if len(id1) != 32 {
		t.Fatalf("GroupID length = %d, want 32 (128-bit hex)", len(id1))
	}
	if id1 == id2 {
		t.Fatal("two generated GroupIDs are identical")
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id1) {
		t.Fatalf("GroupID %q is not 32 lowercase hex characters", id1)
	}
}

func TestGroupCodeFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode() error = %v", err)
		}
		if len(code) != 8 {
			t.Fatalf("GroupCode length = %d, want 8 (code=%q)", len(code), code)
		}
		if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(code) {
			t.Fatalf("GroupCode %q contains non-digit characters", code)
		}
	}
}

func TestGroupCodeWithLeadingZeros(t *testing.T) {
	code := "01234567"
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(code) {
		t.Fatalf("valid leading-zero code %q rejected", code)
	}
	g := &Group{ID: "id", Code: code, Name: "test", HostID: "host", Members: map[string]Member{"host": {ID: "host", Role: RoleHost}}}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate() with leading-zero code = %v", err)
	}
}

func TestValidateRejectsInvalidGroup(t *testing.T) {
	base := func() *Group {
		g, _ := Create("Test", "host-id", Open)
		return g
	}

	tests := []struct {
		name    string
		mutate  func(g *Group)
		wantErr string
	}{
		{
			name:    "empty name",
			mutate:  func(g *Group) { g.Name = "   " },
			wantErr: "name",
		},
		{
			name:    "empty host id",
			mutate:  func(g *Group) { g.HostID = "" },
			wantErr: "host",
		},
		{
			name:    "empty group id",
			mutate:  func(g *Group) { g.ID = "" },
			wantErr: "id",
		},
		{
			name:    "invalid group code length",
			mutate:  func(g *Group) { g.Code = "1234567" },
			wantErr: "8 digits",
		},
		{
			name:    "group code non-digit",
			mutate:  func(g *Group) { g.Code = "1234567a" },
			wantErr: "8 digits",
		},
		{
			name:    "host not in members",
			mutate:  func(g *Group) { delete(g.Members, g.HostID) },
			wantErr: "host must be a member",
		},
		{
			name: "duplicate host role",
			mutate: func(g *Group) {
				g.Members["other"] = Member{ID: "other", Role: RoleHost}
			},
			wantErr: "exactly one host",
		},
		{
			name: "member map key mismatch",
			mutate: func(g *Group) {
				g.Members["other"] = Member{ID: "different", Role: RoleMember}
			},
			wantErr: "does not match",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := base()
			tt.mutate(g)
			err := g.Validate()
			if err == nil {
				t.Fatalf("Validate() succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestAddMember(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}
	if !g.HasMember("member-1") {
		t.Fatal("HasMember(member-1) = false, want true")
	}
	m, exists := g.GetMember("member-1")
	if !exists {
		t.Fatal("GetMember(member-1) not found")
	}
	if m.Role != RoleMember {
		t.Fatalf("new member role = %v, want %v", m.Role, RoleMember)
	}
	if m.Name != "Member One" {
		t.Fatalf("member name = %q, want %q", m.Name, "Member One")
	}
}

func TestAddDuplicateMemberRejected(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	if err := g.AddMember("member-1", "First"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Second"); err == nil {
		t.Fatal("AddMember() with duplicate ID succeeded")
	}
}

func TestIsHost(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	if !g.IsHost("host-id") {
		t.Fatal("IsHost(host-id) = false, want true")
	}
	if err := g.AddMember("member-1", "Member"); err != nil {
		t.Fatal(err)
	}
	if g.IsHost("member-1") {
		t.Fatal("IsHost(member-1) = true, want false")
	}
}

func TestRemoveMember(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	if err := g.AddMember("member-1", "Member"); err != nil {
		t.Fatal(err)
	}
	if err := g.RemoveMember("member-1"); err != nil {
		t.Fatalf("RemoveMember(member-1) error = %v", err)
	}
	if g.HasMember("member-1") {
		t.Fatal("HasMember(member-1) = true after removal")
	}
}

func TestRemoveMemberNotFound(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	if err := g.RemoveMember("nobody"); err == nil {
		t.Fatal("RemoveMember(nobody) succeeded, want error")
	}
}

func TestRemoveHostRejected(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	if err := g.RemoveMember("host-id"); err == nil {
		t.Fatal("RemoveMember(host-id) succeeded, want error")
	}
}

func TestCloseGroup(t *testing.T) {
	g, _ := Create("Test", "host-id", Open)
	g.Close()
	if !g.Closed {
		t.Fatal("Closed = false after Close()")
	}
	if err := g.AddMember("member-1", "Member"); err == nil {
		t.Fatal("AddMember() on closed group succeeded")
	}
}

func TestPendingRequests(t *testing.T) {
	pr := NewPendingRequests()
	req := JoinRequest{
		GroupID:       "group-id",
		RequesterID:   "requester-1",
		RequesterName: "Requester One",
		Status:        Pending,
	}
	if err := pr.Add(req, false); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if pr.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", pr.Count())
	}

	got, exists := pr.GetPending("requester-1")
	if !exists {
		t.Fatal("GetPending(requester-1) not found")
	}
	if got.Status != Pending {
		t.Fatalf("request status = %v, want %v", got.Status, Pending)
	}
	if got.RequesterName != "Requester One" {
		t.Fatalf("request name = %q, want %q", got.RequesterName, "Requester One")
	}

	pr.Remove("requester-1")
	if pr.Count() != 0 {
		t.Fatalf("Count() after remove = %d, want 0", pr.Count())
	}
	if _, exists := pr.GetPending("requester-1"); exists {
		t.Fatal("GetPending after remove returned true")
	}
}

func TestPendingRequestsDuplicateRejected(t *testing.T) {
	pr := NewPendingRequests()
	req := JoinRequest{
		GroupID:       "group-id",
		RequesterID:   "requester-1",
		RequesterName: "Requester One",
		Status:        Pending,
	}
	if err := pr.Add(req, false); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}

	// Duplicate pending request from same requester must be rejected.
	req2 := JoinRequest{
		GroupID:       "group-id",
		RequesterID:   "requester-1",
		RequesterName: "Requester One Updated",
		Status:        Pending,
	}
	if err := pr.Add(req2, false); err == nil {
		t.Fatal("duplicate Add() succeeded, want error")
	}

	// Count must remain 1.
	if pr.Count() != 1 {
		t.Fatalf("Count() after duplicate = %d, want 1", pr.Count())
	}

	// Existing request must be unchanged.
	got, exists := pr.GetPending("requester-1")
	if !exists {
		t.Fatal("GetPending(requester-1) not found after duplicate rejection")
	}
	if got.RequesterName != "Requester One" {
		t.Fatalf("request name after duplicate = %q, want %q", got.RequesterName, "Requester One")
	}
}

func TestPendingRequestsRejectsMember(t *testing.T) {
	pr := NewPendingRequests()
	req := JoinRequest{
		GroupID:       "group-id",
		RequesterID:   "requester-1",
		RequesterName: "Requester One",
		Status:        Pending,
	}
	if err := pr.Add(req, true); err == nil {
		t.Fatal("Add() for existing member succeeded")
	}
}

func TestPendingRequestsRejectsNonPending(t *testing.T) {
	pr := NewPendingRequests()
	req := JoinRequest{
		GroupID:       "group-id",
		RequesterID:   "requester-1",
		RequesterName: "Requester One",
		Status:        Accepted,
	}
	if err := pr.Add(req, false); err == nil {
		t.Fatal("Add() with Accepted status succeeded")
	}
}
