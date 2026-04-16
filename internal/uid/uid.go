package uid

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a random 8-byte hex string (16 chars).
func New() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
