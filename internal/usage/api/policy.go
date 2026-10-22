package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

type Grant struct {
	Environment  string   `json:"environment"`
	HostID       string   `json:"host_id"`
	SessionRoots []string `json:"session_roots"`
}
type RemoteLogAccess struct {
	Mode   string  `json:"mode"`
	Grants []Grant `json:"grants"`
}
type Policy struct {
	Enabled         bool            `json:"enabled"`
	RemoteLogAccess RemoteLogAccess `json:"remote_log_access"`
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
	return Policy{Enabled: true, RemoteLogAccess: RemoteLogAccess{Mode: "disabled", Grants: []Grant{}}}
}

var leafRoot = regexp.MustCompile(`(?i)(/sessions/[0-9]{4}/[0-9]{2}/[0-9]{2}|/terminal-agents/[a-z0-9-]+/[a-z0-9-]+/sessions|/\.dsh/sessions/[^/]+/[a-z0-9-]+)$`)

func SafeRoot(root string) bool {
	s := strings.ReplaceAll(root, `\`, "/")
	if len(s) > 2048 || strings.ContainsAny(s, "*?\x00\r\n") || strings.Contains(s, "/../") || strings.Contains(s, "/./") || strings.HasSuffix(s, "/") {
		return false
	}
	abs := strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") || len(s) > 3 && s[1] == ':' && s[2] == '/'
	return abs && leafRoot.MatchString(s)
}
func (p Policy) Validate() error {
	if p.RemoteLogAccess.Mode != "disabled" && p.RemoteLogAccess.Mode != "temporary" || p.RemoteLogAccess.Grants == nil || p.RemoteLogAccess.Mode == "disabled" && len(p.RemoteLogAccess.Grants) > 0 {
		return errors.New("invalid usage_policy")
	}
	for _, g := range p.RemoteLogAccess.Grants {
		if g.Environment == "" || !strings.HasPrefix(g.HostID, "sha256:") || len(g.HostID) != 71 || len(g.SessionRoots) == 0 {
			return errors.New("invalid usage grant")
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(g.HostID, "sha256:")); err != nil {
			return err
		}
		for _, root := range g.SessionRoots {
			if !SafeRoot(root) {
				return errors.New("unsafe usage root")
			}
		}
	}
	return nil
}
func (p Policy) Digest() string {
	raw, _ := json.Marshal(p)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (p Policy) Allows(environment, host, root string) bool {
	if !p.Enabled || p.RemoteLogAccess.Mode != "temporary" {
		return false
	}
	for _, g := range p.RemoteLogAccess.Grants {
		if g.Environment != environment || g.HostID != host {
			continue
		}
		for _, r := range g.SessionRoots {
			if strings.EqualFold(strings.ReplaceAll(r, `\`, "/"), strings.ReplaceAll(root, `\`, "/")) {
				return true
			}
		}
	}
	return false
}
