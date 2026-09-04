package interview

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func newSessionID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return time.Now().UTC().Format("20060102T150405.000000000")
}
