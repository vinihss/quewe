// Package id implements the identity port.
package id

import (
	"crypto/rand"
	"encoding/hex"

	portid "github.com/vinihss/quewe/services/quewe/internal/ports/id"
)

// Random generates 128-bit random identifiers as hex.
type Random struct{}

var _ portid.Generator = Random{}

// New returns a new identifier.
func (Random) New() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
