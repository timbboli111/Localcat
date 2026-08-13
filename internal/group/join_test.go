package group

import (
	"testing"
)

func TestJoinRequestLifecycle(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	// Create pending request
	result, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "PENDING" {
		t.Fatalf("status = %q, want PENDING", result.Status)
	}

	// Accept
	if err := m.AcceptJoinRequest(g.ID, "requester-1", "host-1"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetGroup(g.ID)
	if !got.HasMember("requester-1") {
		t.Fatal("requester not added as member after accept")
	}
	member, _ := got.GetMember("requester-1")
	if member.Role != RoleMember {
		t.Fatalf("member role = %v, want RoleMember", member.Role)
	}
	if member.Name != "Requester One" {
		t.Fatalf("member name = %q, want Requester One", member.Name)
	}
}

func TestJoinRequestRejectLifecycle(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	_, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One")
	if err != nil {
		t.Fatal(err)
	}

	if err := m.RejectJoinRequest(g.ID, "requester-1", "host-1"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetGroup(g.ID)
	if got.HasMember("requester-1") {
		t.Fatal("requester became member after reject")
	}
	pr, _ := m.GetPendingRequests(g.ID)
	if pr.Count() != 0 {
		t.Fatalf("pending count after reject = %d, want 0", pr.Count())
	}
}

func TestJoinRequestGroupCodeMismatch(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	g.Code = "11111111"
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	// Try to find with wrong code
	_, err := m.GetGroupByCode("99999999")
	if err == nil {
		t.Fatal("GetGroupByCode() with wrong code succeeded")
	}
	if err != ErrGroupNotFound {
		t.Fatalf("GetGroupByCode() error = %v, want ErrGroupNotFound", err)
	}
}

func TestJoinClosedGroupRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	g.Close()

	_, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One")
	if err == nil {
		t.Fatal("RequestJoinLockedGroup() on closed group succeeded")
	}
}

func TestRequestJoinOpenGroupFails(t *testing.T) {
	m := NewManager()
	g, _ := Create("Open Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	// RequestJoinLockedGroup should fail for OPEN group
	_, err := RequestJoinLockedGroup(m, g.ID, "requester-1", "Requester One")
	if err == nil {
		t.Fatal("RequestJoinLockedGroup() on OPEN group succeeded")
	}
}

func TestJoinLockedGroupDirectFails(t *testing.T) {
	m := NewManager()
	g, _ := Create("Locked Group", "host-1", Locked)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	// JoinOpenGroup should fail for LOCKED group
	_, err := JoinOpenGroup(m, g.ID, "requester-1", "Requester One")
	if err == nil {
		t.Fatal("JoinOpenGroup() on LOCKED group succeeded")
	}
}
