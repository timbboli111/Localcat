package group

import (
	"errors"
	"strings"
	"testing"
)

func TestManagerAddGroup(t *testing.T) {
	m := NewManager()
	g, err := Create("Test Group", "host-1", Open)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.AddGroup(g); err != nil {
		t.Fatalf("AddGroup() error = %v", err)
	}
	got, exists := m.GetGroup(g.ID)
	if !exists {
		t.Fatal("GetGroup() not found")
	}
	if got.Name != "Test Group" {
		t.Fatalf("GetGroup().Name = %q, want Test Group", got.Name)
	}
}

func TestManagerAddDuplicateGroup(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := m.AddGroup(g); err == nil {
		t.Fatal("AddGroup() duplicate succeeded")
	}
}

func TestManagerGetGroupByCode(t *testing.T) {
	m := NewManager()
	g1, _ := Create("Alpha", "host-1", Open)
	g1.Code = "11111111"
	g2, _ := Create("Beta", "host-2", Locked)
	g2.Code = "22222222"
	if err := m.AddGroup(g1); err != nil {
		t.Fatal(err)
	}
	if err := m.AddGroup(g2); err != nil {
		t.Fatal(err)
	}

	got, err := m.GetGroupByCode("22222222")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != g2.ID {
		t.Fatalf("GetGroupByCode() ID = %q, want %q", got.ID, g2.ID)
	}
	if _, err := m.GetGroupByCode("99999999"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("GetGroupByCode() error = %v, want ErrGroupNotFound", err)
	}
}

func TestManagerGetGroupByCodeAmbiguous(t *testing.T) {
	m := NewManager()
	g1, _ := Create("A", "host-1", Open)
	g1.Code = "12345678"
	g2, _ := Create("B", "host-2", Open)
	g2.Code = "12345678"
	if err := m.AddGroup(g1); err != nil {
		t.Fatal(err)
	}
	if err := m.AddGroup(g2); err != nil {
		t.Fatal(err)
	}
	_, err := m.GetGroupByCode("12345678")
	if !errors.Is(err, ErrGroupCodeAmbiguous) {
		t.Fatalf("GetGroupByCode() error = %v, want ErrGroupCodeAmbiguous", err)
	}
}

func TestJoinOpenGroup(t *testing.T) {
	m := NewManager()
	g, _ := Create("Open Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	result, err := JoinOpenGroup(m, g.ID, "requester-1", "Requester One")
	if err != nil {
		t.Fatalf("JoinOpenGroup() error = %v", err)
	}
	if result.Status != "MEMBER" {
		t.Fatalf("JoinOpenGroup() status = %q, want MEMBER", result.Status)
	}
	got, _ := m.GetGroup(g.ID)
	if !got.HasMember("requester-1") {
		t.Fatal("HasMember(requester-1) = false, want true")
	}
	member, _ := got.GetMember("requester-1")
	if member.Role != RoleMember {
		t.Fatalf("member role = %v, want RoleMember", member.Role)
	}
}

func TestJoinLockedGroupCreatesPending(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	result, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One")
	if err != nil {
		t.Fatalf("RequestJoinLockedGroup() error = %v", err)
	}
	if result.Status != "PENDING" {
		t.Fatalf("RequestJoinLockedGroup() status = %q, want PENDING", result.Status)
	}
	pr, _ := m.GetPendingRequests(g.ID)
	if pr.Count() != 1 {
		t.Fatalf("pending count = %d, want 1", pr.Count())
	}
	if _, exists := pr.GetPending("requester-1"); !exists {
		t.Fatal("GetPending(requester-1) not found")
	}
}

func TestHostAcceptJoinRequest(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}

	if err := m.AcceptJoinRequest(g.ID, "requester-1", "host-1"); err != nil {
		t.Fatalf("AcceptJoinRequest() error = %v", err)
	}
	got, _ := m.GetGroup(g.ID)
	if !got.HasMember("requester-1") {
		t.Fatal("HasMember(requester-1) = false after accept")
	}
	pr, _ := m.GetPendingRequests(g.ID)
	if pr.Count() != 0 {
		t.Fatalf("pending count after accept = %d, want 0", pr.Count())
	}
}

func TestHostRejectJoinRequest(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}

	if err := m.RejectJoinRequest(g.ID, "requester-1", "host-1"); err != nil {
		t.Fatalf("RejectJoinRequest() error = %v", err)
	}
	got, _ := m.GetGroup(g.ID)
	if got.HasMember("requester-1") {
		t.Fatal("HasMember(requester-1) = true after reject")
	}
	pr, _ := m.GetPendingRequests(g.ID)
	if pr.Count() != 0 {
		t.Fatalf("pending count after reject = %d, want 0", pr.Count())
	}
}

func TestDuplicatePendingRequestRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err == nil {
		t.Fatal("duplicate RequestJoinLockedGroup() succeeded")
	}
}

func TestExistingMemberCannotJoinAgain(t *testing.T) {
	m := NewManager()
	g, _ := Create("Open Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if _, err := JoinOpenGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}
	if _, err := JoinOpenGroup(m, g.ID, "requester-1", "Requester One"); err == nil {
		t.Fatal("JoinOpenGroup() for existing member succeeded")
	}
}

func TestClosedGroupRejectsJoin(t *testing.T) {
	m := NewManager()
	g, _ := Create("Closed Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	g.Close()
	if _, err := JoinOpenGroup(m, g.ID, "requester-1", "Requester One"); err == nil {
		t.Fatal("JoinOpenGroup() on closed group succeeded")
	}
}

func TestGroupCodeMismatchRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	_, err := m.GetGroupByCode("99999999")
	if !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("GetGroupByCode() wrong code error = %v, want ErrGroupNotFound", err)
	}
}

func TestUnknownGroupIDRejected(t *testing.T) {
	m := NewManager()
	group, exists := m.GetGroup("nonexistent")
	if exists {
		t.Fatalf("GetGroup() returned exists=true for unknown ID, group=%v", group)
	}
	if group != nil {
		t.Fatalf("GetGroup() returned non-nil group for unknown ID: %v", group)
	}
}

func TestNonHostCannotAccept(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}

	err := m.AcceptJoinRequest(g.ID, "requester-1", "member-1")
	if err == nil {
		t.Fatal("AcceptJoinRequest() by non-host succeeded")
	}
	if !strings.Contains(err.Error(), "only the host") {
		t.Fatalf("AcceptJoinRequest() error = %q, want host-only error", err)
	}
}

func TestNonHostCannotReject(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}

	err := m.RejectJoinRequest(g.ID, "requester-1", "member-1")
	if err == nil {
		t.Fatal("RejectJoinRequest() by non-host succeeded")
	}
}

func TestFakeHostRoleCannotAccept(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One"); err != nil {
		t.Fatal(err)
	}

	err := m.AcceptJoinRequest(g.ID, "requester-1", "attacker-id")
	if err == nil {
		t.Fatal("AcceptJoinRequest() by fake host succeeded")
	}
	if !strings.Contains(err.Error(), "only the host") {
		t.Fatalf("AcceptJoinRequest() error = %q, want host-only error", err)
	}
}

func TestEmptyRequesterIDRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Open Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if _, err := JoinOpenGroup(m, g.ID, "", "Name"); err == nil {
		t.Fatal("JoinOpenGroup() with empty requester ID succeeded")
	}
	if _, err := RequestJoinLockedGroup(m, g.ID, "", "Name"); err == nil {
		t.Fatal("RequestJoinLockedGroup() with empty requester ID succeeded")
	}
}

func TestHostCannotBeRemoved(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.RemoveMember("host-1"); err == nil {
		t.Fatal("RemoveMember(host-1) succeeded")
	}
	if !g.HasMember("host-1") {
		t.Fatal("host is no longer a member after RemoveMember attempt")
	}
}

func TestNoHostMigration(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("host-2", "Host Two"); err != nil {
		t.Fatal(err)
	}
	g.Members["host-2"] = Member{ID: "host-2", Name: "Host Two", Role: RoleHost, JoinedAt: TimeNow()}
	if err := g.Validate(); err == nil {
		t.Fatal("Validate() succeeded with two hosts")
	}
	if g.HostID != "host-1" {
		t.Fatalf("HostID = %q, want host-1", g.HostID)
	}
	if !g.IsHost("host-1") {
		t.Fatal("IsHost(host-1) = false, want true")
	}
	if g.IsHost("host-2") {
		t.Fatal("IsHost(host-2) = true, want false")
	}
}

func TestCloseGroupByNonHostRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member"); err != nil {
		t.Fatal(err)
	}
	if err := m.CloseGroup(g.ID, "member-1"); err == nil {
		t.Fatal("CloseGroup() by non-host succeeded")
	}
	if g.Closed {
		t.Fatal("group closed by non-host")
	}
}
