package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
)

const (
	prefDisplayName = "identity.display_name"
	prefDeviceID    = "identity.device_id"
)

// Identity is the stable local identity for this LocalCat installation.
type Identity struct {
	ID          string
	DisplayName string
}

// Load reads the locally configured identity from Fyne preferences.
func Load(prefs fyne.Preferences) (Identity, error) {
	id := strings.TrimSpace(prefs.String(prefDeviceID))
	if id == "" {
		generated, err := newID()
		if err != nil {
			return Identity{}, err
		}
		id = generated
		prefs.SetString(prefDeviceID, id)
	}
	return Identity{ID: id, DisplayName: strings.TrimSpace(prefs.String(prefDisplayName))}, nil
}

// SaveDisplayName persists a human-readable display name without changing the stable device ID.
func SaveDisplayName(prefs fyne.Preferences, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("display name is required")
	}
	prefs.SetString(prefDisplayName, name)
	return nil
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate identity id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
