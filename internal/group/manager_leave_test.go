package group

import (
	"testing"
)

func TestLeaveGroupAsMember(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}

	if err := m.LeaveGroup(g.ID, "member-1"); err != nil {
		t.Fatalf("LeaveGroup() error = %v", err)
	}
	if g.HasMember("member-1") {
		t.Fatal("HasMember(member-1) = true after leave")
	}
}

func TestLeaveGroupAsHostRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	if err := m.LeaveGroup(g.ID, "host-1"); err == nil {
		t.Fatal("LeaveGroup() for host succeeded")
	}
	if !g.HasMember("host-1") {
		t.Fatal("host is no longer a member after LeaveGroup attempt")
	}
}

func TestLeaveGroupNonMemberRejected(t *testing.T) {
	m := NewManager()
	g, _ := Create("Test Group", "host-1", Open)
	if err := m.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	if err := m.LeaveGroup(g.ID, "non-member"); err == nil {
		t.Fatal("LeaveGroup() for non-member succeeded")
	}
}
