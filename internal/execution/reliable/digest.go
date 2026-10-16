package reliable

import (
	"crypto/sha256"
	"encoding/hex"
)

func Digest(wire []byte) string { value := sha256.Sum256(wire); return hex.EncodeToString(value[:]) }
