package group

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Persistence handles loading and saving Group metadata to a JSON file.
type Persistence struct {
	mu   sync.Mutex
	path string
}

// NewPersistence creates a Persistence backed by the given file path.
func NewPersistence(path string) *Persistence {
	return &Persistence{path: path}
}

// Load reads all groups from the persistence file.
// Returns an empty slice if the file does not exist.
func (p *Persistence) Load() ([]*Group, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	b, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Group{}, nil
		}
		return nil, fmt.Errorf("read groups file: %w", err)
	}
	if len(b) == 0 {
		return []*Group{}, nil
	}

	var groups []*Group
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil, fmt.Errorf("decode groups file: %w", err)
	}

	// Ensure all groups have initialized maps
	for _, g := range groups {
		if g.Members == nil {
			g.Members = make(map[string]Member)
		}
	}
	return groups, nil
}

// Save writes all groups to the persistence file.
// Creates parent directories if needed.
func (p *Persistence) Save(groups []*Group) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return fmt.Errorf("create groups directory: %w", err)
	}

	b, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return fmt.Errorf("encode groups: %w", err)
	}
	return os.WriteFile(p.path, b, 0o600)
}

// SaveGroup saves a single group. If the group already exists in the
// file, it is replaced. If not, it is appended.
func (p *Persistence) SaveGroup(g *Group) error {
	groups, err := p.Load()
	if err != nil {
		return err
	}
	found := false
	for i, existing := range groups {
		if existing.ID == g.ID {
			groups[i] = g
			found = true
			break
		}
	}
	if !found {
		groups = append(groups, g)
	}
	return p.Save(groups)
}

// RemoveGroup removes a group from the persistence file.
func (p *Persistence) RemoveGroup(groupID string) error {
	groups, err := p.Load()
	if err != nil {
		return err
	}
	filtered := groups[:0]
	for _, g := range groups {
		if g.ID != groupID {
			filtered = append(filtered, g)
		}
	}
	return p.Save(filtered)
}

// EnsureGroupsDir creates the directory for the groups file if needed.
func EnsureGroupsDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
