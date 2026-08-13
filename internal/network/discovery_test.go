package network

import (
	"encoding/json"
	"testing"
	"time"

	"localcat/internal/group"
)

func TestAnnouncementBackwardCompatible(t *testing.T) {
	oldJSON := `{"app":"LocalCat","id":"peer-1","name":"Alice","port":12345}`
	var ann announcement
	if err := json.Unmarshal([]byte(oldJSON), &ann); err != nil {
		t.Fatalf("unmarshal old announcement: %v", err)
	}
	if ann.App != "LocalCat" || ann.ID != "peer-1" || ann.Name != "Alice" || ann.Port != 12345 {
		t.Fatalf("old announcement parsed incorrectly: %+v", ann)
	}
	if ann.Groups != nil {
		t.Fatalf("old announcement groups = %v, want nil", ann.Groups)
	}
}

func TestAnnouncementWithGroups(t *testing.T) {
	ann := announcement{
		App:  "LocalCat",
		ID:   "peer-1",
		Name: "Alice",
		Port: 12345,
		Groups: []group.GroupAdvertisement{
			{ID: "group-1", Code: "12345678", Name: "Test", HostID: "peer-1", JoinPolicy: group.Open},
		},
	}
	data, err := json.Marshal(ann)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded announcement
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Groups) != 1 {
		t.Fatalf("decoded groups len = %d, want 1", len(decoded.Groups))
	}
	if decoded.Groups[0].Code != "12345678" {
		t.Fatalf("decoded group code = %q, want 12345678", decoded.Groups[0].Code)
	}
	if decoded.Groups[0].JoinPolicy != group.Open {
		t.Fatalf("decoded join policy = %v, want Open", decoded.Groups[0].JoinPolicy)
	}
}

func TestPeerGroupsField(t *testing.T) {
	p := Peer{
		ID:       "peer-1",
		Name:     "Alice",
		Groups:   nil,
		LastSeen: time.Now(),
	}
	if p.Groups != nil {
		t.Fatalf("Peer.Groups = %v, want nil by default", p.Groups)
	}
	p.Groups = []group.GroupAdvertisement{}
	if p.Groups == nil {
		t.Fatal("Peer.Groups should be empty slice, not nil")
	}
}
