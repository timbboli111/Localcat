package group

import (
	"fmt"
	"sync"
)

// GroupAdvertisement is the LAN-discovery metadata for an active group.
// It contains only what is needed to find and identify a group.
type GroupAdvertisement struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	HostID     string     `json:"host_id"`
	JoinPolicy JoinPolicy `json:"join_policy"`
	HostName   string     `json:"host_name,omitempty"`
	HostAddr   string     `json:"host_addr,omitempty"`
	HostPort   int        `json:"host_port,omitempty"`
}

// GroupDiscovery holds a view of advertised groups received via LAN discovery.
// Lifecycle and expiry are managed by the existing peer discovery mechanism;
// this type only stores and searches group metadata.
type GroupDiscovery struct {
	mu     sync.RWMutex
	groups map[string][]GroupAdvertisement // key: GroupCode
}

// NewGroupDiscovery creates an empty GroupDiscovery.
func NewGroupDiscovery() *GroupDiscovery {
	return &GroupDiscovery{groups: make(map[string][]GroupAdvertisement)}
}

// Upsert adds or updates a group advertisement.
// Advertisements are keyed by GroupID; if the same GroupID is already
// present for the same GroupCode, it is replaced.
func (d *GroupDiscovery) Upsert(ad GroupAdvertisement) {
	d.mu.Lock()
	defer d.mu.Unlock()
	code := ad.Code
	existing := d.groups[code]
	for i, g := range existing {
		if g.ID == ad.ID {
			existing[i] = ad
			d.groups[code] = existing
			return
		}
	}
	d.groups[code] = append(existing, ad)
}

// Remove removes a group advertisement by GroupID.
func (d *GroupDiscovery) Remove(groupID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for code, list := range d.groups {
		filtered := list[:0]
		for _, g := range list {
			if g.ID != groupID {
				filtered = append(filtered, g)
			}
		}
		if len(filtered) == 0 {
			delete(d.groups, code)
		} else {
			d.groups[code] = filtered
		}
	}
}

// Clear removes all advertisements.
func (d *GroupDiscovery) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups = make(map[string][]GroupAdvertisement)
}

// FindByCode searches for a group by its 8-digit GroupCode.
//
// Returns:
//   - (GroupAdvertisement, nil) if exactly one match is found
//   - (GroupAdvertisement{}, ErrGroupNotFound) if no match
//   - (GroupAdvertisement{}, ErrGroupCodeAmbiguous) if more than one
//     distinct GroupID shares the same GroupCode
func (d *GroupDiscovery) FindByCode(code string) (GroupAdvertisement, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	list := d.groups[code]
	if len(list) == 0 {
		return GroupAdvertisement{}, ErrGroupNotFound
	}
	if len(list) == 1 {
		return list[0], nil
	}
	// More than one entry — check if all share the same GroupID.
	firstID := list[0].ID
	for _, g := range list[1:] {
		if g.ID != firstID {
			return GroupAdvertisement{}, ErrGroupCodeAmbiguous
		}
	}
	return list[0], nil
}

// FindByID searches for a group by its internal GroupID.
func (d *GroupDiscovery) FindByID(groupID string) (GroupAdvertisement, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, list := range d.groups {
		for _, g := range list {
			if g.ID == groupID {
				return g, true
			}
		}
	}
	return GroupAdvertisement{}, false
}

// All returns a snapshot of all advertised groups.
func (d *GroupDiscovery) All() []GroupAdvertisement {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []GroupAdvertisement
	seen := make(map[string]bool)
	for _, list := range d.groups {
		for _, g := range list {
			if !seen[g.ID] {
				seen[g.ID] = true
				out = append(out, g)
			}
		}
	}
	return out
}

// Count returns the number of distinct groups currently known.
func (d *GroupDiscovery) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	seen := make(map[string]bool)
	for _, list := range d.groups {
		for _, g := range list {
			seen[g.ID] = true
		}
	}
	return len(seen)
}

// Errors for group discovery.
var (
	ErrGroupNotFound      = fmt.Errorf("group not found")
	ErrGroupCodeAmbiguous = fmt.Errorf("group code is ambiguous")
)
