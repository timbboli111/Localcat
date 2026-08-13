package group

import (
	"strings"
	"testing"
)

func TestRelayRegisterGroup(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatalf("RegisterGroup() error = %v", err)
	}
	if err := r.RegisterGroup(g); err == nil {
		t.Fatal("RegisterGroup() duplicate succeeded")
	}
}

func TestRelaySetMemberPeer(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMemberPeer(g.ID, "member-1", "192.168.1.10", 9000); err != nil {
		t.Fatalf("SetMemberPeer() error = %v", err)
	}
	if err := r.SetMemberPeer(g.ID, "non-member", "192.168.1.11", 9001); err == nil {
		t.Fatal("SetMemberPeer() for non-member succeeded")
	}
}

func TestRelayValidateMemberCanSend(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}

	// Host can send
	if err := r.ValidateMemberCanSend(g.ID, "host-1"); err != nil {
		t.Fatalf("ValidateMemberCanSend(host) error = %v", err)
	}

	// Member can send
	if err := r.ValidateMemberCanSend(g.ID, "member-1"); err != nil {
		t.Fatalf("ValidateMemberCanSend(member) error = %v", err)
	}

	// Non-member cannot send
	if err := r.ValidateMemberCanSend(g.ID, "non-member"); err == nil {
		t.Fatal("ValidateMemberCanSend(non-member) succeeded")
	}

	// Unknown group
	if err := r.ValidateMemberCanSend("unknown", "host-1"); err == nil {
		t.Fatal("ValidateMemberCanSend(unknown group) succeeded")
	}
}

func TestRelayValidateMemberCanSendClosedGroup(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatal(err)
	}
	g.Close()
	err := r.ValidateMemberCanSend(g.ID, "host-1")
	if err == nil {
		t.Fatal("ValidateMemberCanSend() on closed group succeeded")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("error = %q, want closed", err)
	}
}

func TestRelayGetRelayTargets(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-2", "Member Two"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMemberPeer(g.ID, "host-1", "192.168.1.1", 9000); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMemberPeer(g.ID, "member-1", "192.168.1.2", 9001); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMemberPeer(g.ID, "member-2", "192.168.1.3", 9002); err != nil {
		t.Fatal(err)
	}

	// Host sends — targets should be member-1 and member-2, not host
	targets, err := r.GetRelayTargets(g.ID, "host-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets len = %d, want 2", len(targets))
	}
	for _, relayPeer := range targets {
		if relayPeer.Address == "192.168.1.1" && relayPeer.Port == 9000 {
			t.Fatalf("host is in relay targets")
		}
	}

	// Member-1 sends — targets should include host and member-2, not member-1
	targets, err = r.GetRelayTargets(g.ID, "member-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets len = %d, want 2", len(targets))
	}
	for _, relayPeer := range targets {
		if relayPeer.Address == "192.168.1.2" && relayPeer.Port == 9001 {
			t.Fatalf("sender member-1 is in relay targets")
		}
	}
}

func TestRelayRemoveMemberPeer(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member One"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMemberPeer(g.ID, "member-1", "192.168.1.2", 9001); err != nil {
		t.Fatal(err)
	}
	r.RemoveMemberPeer(g.ID, "member-1")
	targets, err := r.GetRelayTargets(g.ID, "host-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("targets after remove = %d, want 0", len(targets))
	}
}

func TestRelayUnregisterGroup(t *testing.T) {
	r := NewRelay()
	g, _ := Create("Test Group", "host-1", Open)
	if err := r.RegisterGroup(g); err != nil {
		t.Fatal(err)
	}
	r.UnregisterGroup(g.ID)
	if err := r.ValidateMemberCanSend(g.ID, "host-1"); err == nil {
		t.Fatal("ValidateMemberCanSend() after unregister succeeded")
	}
}
