// Package idgen provides small, dependency-light IDs for IAM control-plane
// records. It is intentionally local to aisphere-iam so kernel remains free of
// Aisphere-specific ID policies.
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// New returns an ID in the form <prefix>_<unix-nano>_<random-hex>.
// The format is stable enough for logs and debugging, while still avoiding
// database round-trips for control-plane record creation.
func New(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "id"
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + timestamp()
	}
	return prefix + "_" + timestamp() + "_" + hex.EncodeToString(b[:])
}

func timestamp() string {
	return time.Now().UTC().Format("20060102150405_000000000")
}
