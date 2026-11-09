package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// Policy enables collection for local and verified remote Run sessions.
// Remote log access is derived from the original binding, without saved grants.
type Policy struct {
	Enabled bool `json:"enabled"`
}

const CollectionPermitKey = "collection-permit"

var ErrCollectionRevoked = errors.New("usage_collection_revoked")

// CollectionPermit fences a writer's observation independently of Run controls.
// Revocation leaves cleanup and immutable history queries available.
type CollectionPermit struct {
	Instance     string `json:"instance"`
	PolicyDigest string `json:"policy_digest"`
	Active       bool   `json:"active"`
}

func DefaultPolicy() Policy {
	return Policy{Enabled: true}
}

var leafRoot = regexp.MustCompile(`(?i)(/sessions/[0-9]{4}/[0-9]{2}/[0-9]{2}|/terminal-agents/[a-z0-9-]+/[a-z0-9-]+/sessions)$`)

func SafeRoot(root string) bool {
	s := strings.ReplaceAll(root, `\`, "/")
	if len(s) > 2048 || strings.ContainsAny(s, "*?\x00\r\n") || strings.Contains(s, "/../") || strings.Contains(s, "/./") || strings.HasSuffix(s, "/") {
		return false
	}
	abs := strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") || len(s) > 3 && s[1] == ':' && s[2] == '/'
	return abs && leafRoot.MatchString(s)
}
func (p Policy) Digest() string {
	raw, _ := json.Marshal(p)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

// Allows accepts only a verified environment, stable paired host fingerprint and
// a narrow session root. The caller also verifies the original session and path.
func (p Policy) Allows(environment, host, root string) bool {
	if !p.Enabled || environment == "" || !strings.HasPrefix(host, "sha256:") || len(host) != 71 || !SafeRoot(root) {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(host, "sha256:"))
	return err == nil
}
