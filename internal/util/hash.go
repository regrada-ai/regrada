package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func HashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func ShortHash(value string) string {
	full := HashString(value)
	if len(full) < 8 {
		return full
	}
	return full[:8]
}

func NewID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return ShortHash(valueFromRandFailure())
	}
	return hex.EncodeToString(buf)
}

func valueFromRandFailure() string {
	return "regrada"
}
