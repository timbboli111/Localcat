package group

import (
	"errors"
	"path/filepath"
	"testing"

	"localcat/internal/history"
)

func newTestStore(t *testing.T) (*history.Store, error) {
	t.Helper()
	return history.Open(filepath.Join(t.TempDir(), "history.json"))
}

func TestGroupAdvertisementFormat(t *testing.T) {
	ad := GroupAdvertisement{
		ID:         "group-id-1",
		Code:       "12345678",
		Name:       "Test Group",
		HostID:     "host-1",
		JoinPolicy: Open,
	}
	if ad.ID == "" {
		t.Fatal("advertisement ID is empty")
	}
	if ad.Code != "12345678" {
		t.Fatalf("advertisement code = %q, want 12345678", ad.Code)
	}
	if ad.HostID != "host-1" {
		t.Fatalf("advertisement host = %q, want host-1", ad.HostID)
	}
	if ad.JoinPolicy != Open {
		t.Fatalf("advertisement policy = %v, want Open", ad.JoinPolicy)
	}
}

func TestJoinPolicyDistinction(t *testing.T) {
	if Open == Locked {
		t.Fatal("Open and Locked have same value")
	}
	if Open.String() != "OPEN" {
		t.Fatalf("Open.String() = %q, want OPEN", Open.String())
	}
	if Locked.String() != "LOCKED" {
		t.Fatalf("Locked.String() = %q, want LOCKED", Locked.String())
	}
}

func TestFindByCodeFound(t *testing.T) {
	d := NewGroupDiscovery()
	ad := GroupAdvertisement{
		ID:         "group-1",
		Code:       "11111111",
		Name:       "Alpha",
		HostID:     "host-1",
		JoinPolicy: Open,
	}
	d.Upsert(ad)

	got, err := d.FindByCode("11111111")
	if err != nil {
		t.Fatalf("FindByCode() error = %v", err)
	}
	if got.ID != "group-1" {
		t.Fatalf("FindByCode() ID = %q, want group-1", got.ID)
	}
	if got.Name != "Alpha" {
		t.Fatalf("FindByCode() Name = %q, want Alpha", got.Name)
	}
}

func TestFindByCodeNotFound(t *testing.T) {
	d := NewGroupDiscovery()
	_, err := d.FindByCode("99999999")
	if !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("FindByCode() error = %v, want ErrGroupNotFound", err)
	}
}

func TestFindByCodeAmbiguous(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-a", Code: "12345678", Name: "A", HostID: "host-a"})
	d.Upsert(GroupAdvertisement{ID: "group-b", Code: "12345678", Name: "B", HostID: "host-b"})

	_, err := d.FindByCode("12345678")
	if !errors.Is(err, ErrGroupCodeAmbiguous) {
		t.Fatalf("FindByCode() error = %v, want ErrGroupCodeAmbiguous", err)
	}
}

func TestFindByCodeSameGroupIDNotAmbiguous(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "12345678", Name: "Same", HostID: "host-1"})
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "12345678", Name: "Same Updated", HostID: "host-1"})

	got, err := d.FindByCode("12345678")
	if err != nil {
		t.Fatalf("FindByCode() error = %v", err)
	}
	if got.ID != "group-1" {
		t.Fatalf("FindByCode() ID = %q, want group-1", got.ID)
	}
	if got.Name != "Same Updated" {
		t.Fatalf("FindByCode() Name = %q, want Same Updated", got.Name)
	}
}

func TestRemoveGroup(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "11111111", Name: "One"})
	d.Upsert(GroupAdvertisement{ID: "group-2", Code: "22222222", Name: "Two"})

	d.Remove("group-1")
	if _, err := d.FindByCode("11111111"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("FindByCode after Remove = %v, want ErrGroupNotFound", err)
	}
	if got, err := d.FindByCode("22222222"); err != nil || got.ID != "group-2" {
		t.Fatalf("FindByCode after Remove other = %v, %v", got, err)
	}
}

func TestRemoveGroupWithCollision(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-a", Code: "12345678", Name: "A"})
	d.Upsert(GroupAdvertisement{ID: "group-b", Code: "12345678", Name: "B"})

	d.Remove("group-a")
	got, err := d.FindByCode("12345678")
	if err != nil {
		t.Fatalf("FindByCode after Remove = %v", err)
	}
	if got.ID != "group-b" {
		t.Fatalf("remaining group ID = %q, want group-b", got.ID)
	}
}

func TestFindByID(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "11111111", Name: "One"})
	d.Upsert(GroupAdvertisement{ID: "group-2", Code: "22222222", Name: "Two"})

	got, found := d.FindByID("group-2")
	if !found {
		t.Fatal("FindByID(group-2) not found")
	}
	if got.Name != "Two" {
		t.Fatalf("FindByID Name = %q, want Two", got.Name)
	}
	if _, found := d.FindByID("nonexistent"); found {
		t.Fatal("FindByID(nonexistent) found")
	}
}

func TestAllAndCount(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "11111111", Name: "One"})
	d.Upsert(GroupAdvertisement{ID: "group-2", Code: "22222222", Name: "Two"})
	d.Upsert(GroupAdvertisement{ID: "group-3", Code: "33333333", Name: "Three"})

	all := d.All()
	if len(all) != 3 {
		t.Fatalf("All() len = %d, want 3", len(all))
	}
	if d.Count() != 3 {
		t.Fatalf("Count() = %d, want 3", d.Count())
	}
}

func TestUpsertSameIDUpdates(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "11111111", Name: "Old"})
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "11111111", Name: "New"})

	got, err := d.FindByCode("11111111")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "New" {
		t.Fatalf("Name after upsert = %q, want New", got.Name)
	}
	if d.Count() != 1 {
		t.Fatalf("Count after upsert same ID = %d, want 1", d.Count())
	}
}

func TestClear(t *testing.T) {
	d := NewGroupDiscovery()
	d.Upsert(GroupAdvertisement{ID: "group-1", Code: "11111111", Name: "One"})
	d.Upsert(GroupAdvertisement{ID: "group-2", Code: "22222222", Name: "Two"})

	d.Clear()
	if d.Count() != 0 {
		t.Fatalf("Count after Clear = %d, want 0", d.Count())
	}
	if _, err := d.FindByCode("11111111"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("FindByCode after Clear = %v, want ErrGroupNotFound", err)
	}
}
