package group

import (
	"path/filepath"
	"testing"
)

func TestPersistenceSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	p := NewPersistence(path)

	g1, _ := Create("Group A", "host-a", Open)
	g1.Code = "11111111"
	g2, _ := Create("Group B", "host-b", Locked)
	g2.Code = "22222222"

	if err := p.Save([]*Group{g1, g2}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := p.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded groups = %d, want 2", len(loaded))
	}

	// Find by ID
	var foundA, foundB bool
	for _, g := range loaded {
		switch g.ID {
		case g1.ID:
			foundA = true
			if g.Code != "11111111" {
				t.Fatalf("Group A code = %q, want 11111111", g.Code)
			}
			if g.JoinPolicy != Open {
				t.Fatalf("Group A policy = %v, want Open", g.JoinPolicy)
			}
			if g.HostID != "host-a" {
				t.Fatalf("Group A host = %q, want host-a", g.HostID)
			}
			if len(g.Members) != 1 {
				t.Fatalf("Group A members = %d, want 1", len(g.Members))
			}
			if _, exists := g.Members["host-a"]; !exists {
				t.Fatal("Group A host not in members")
			}
			if g.Members["host-a"].Role != RoleHost {
				t.Fatalf("Group A host role = %v, want RoleHost", g.Members["host-a"].Role)
			}
		case g2.ID:
			foundB = true
			if g.Code != "22222222" {
				t.Fatalf("Group B code = %q, want 22222222", g.Code)
			}
			if g.JoinPolicy != Locked {
				t.Fatalf("Group B policy = %v, want Locked", g.JoinPolicy)
			}
		}
	}
	if !foundA || !foundB {
		t.Fatal("not all groups loaded")
	}
}

func TestPersistenceSaveGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	p := NewPersistence(path)

	g, _ := Create("Test Group", "host-1", Open)
	if err := p.SaveGroup(g); err != nil {
		t.Fatalf("SaveGroup() error = %v", err)
	}

	// Update and save again
	g.Name = "Updated Name"
	if err := p.SaveGroup(g); err != nil {
		t.Fatalf("SaveGroup() updated error = %v", err)
	}

	loaded, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded groups = %d, want 1", len(loaded))
	}
	if loaded[0].Name != "Updated Name" {
		t.Fatalf("loaded name = %q, want Updated Name", loaded[0].Name)
	}
}

func TestPersistenceRemoveGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	p := NewPersistence(path)

	g1, _ := Create("Group A", "host-a", Open)
	g2, _ := Create("Group B", "host-b", Open)

	if err := p.Save([]*Group{g1, g2}); err != nil {
		t.Fatal(err)
	}

	if err := p.RemoveGroup(g1.ID); err != nil {
		t.Fatalf("RemoveGroup() error = %v", err)
	}

	loaded, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded groups after remove = %d, want 1", len(loaded))
	}
	if loaded[0].ID != g2.ID {
		t.Fatalf("remaining group ID = %q, want %q", loaded[0].ID, g2.ID)
	}
}

func TestPersistenceLoadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	p := NewPersistence(path)

	loaded, err := p.Load()
	if err != nil {
		t.Fatalf("Load() empty file error = %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded groups = %d, want 0", len(loaded))
	}
}

func TestPersistenceMemberSnapshotPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	p := NewPersistence(path)

	g, _ := Create("Test Group", "host-1", Open)
	if err := g.SetHostDisplayName("Host Display Name"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember("member-1", "Member Display Name"); err != nil {
		t.Fatal(err)
	}

	if err := p.Save([]*Group{g}); err != nil {
		t.Fatal(err)
	}

	loaded, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded = %d, want 1", len(loaded))
	}
	if loaded[0].Members["host-1"].Name != "Host Display Name" {
		t.Fatalf("host name = %q, want Host Display Name", loaded[0].Members["host-1"].Name)
	}
	if loaded[0].Members["member-1"].Name != "Member Display Name" {
		t.Fatalf("member name = %q, want Member Display Name", loaded[0].Members["member-1"].Name)
	}
	if loaded[0].Members["member-1"].Role != RoleMember {
		t.Fatalf("member role = %v, want RoleMember", loaded[0].Members["member-1"].Role)
	}
}
