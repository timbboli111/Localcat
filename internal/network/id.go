package network

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "LocalCat"
	}
	return hex.EncodeToString(buf)
}
