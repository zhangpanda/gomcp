package uid

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a random 8-byte hex string (16 chars).
func New() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("uid: crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
