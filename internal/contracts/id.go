package contracts

import (
	"crypto/rand"
	"encoding/hex"
)

func randomID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("system randomness unavailable")
	}
	return hex.EncodeToString(value[:])
}
