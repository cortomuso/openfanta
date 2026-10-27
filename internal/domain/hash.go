package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex returns the lowercase hex sha256 of b.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
