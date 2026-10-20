package codexworker

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const authenticationSocketLimit = 106

// AuthenticationTransport is public, frozen execution resource data. Its short
// directory is independent of Git administration and never shared by attempts.
type AuthenticationTransport struct {
	StateDirectory string                 `json:"state_directory"`
	Directory      string                 `json:"directory"`
	Workspace      string                 `json:"workspace"`
	Remote         string                 `json:"remote"`
	Identity       PolicyResourceIdentity `json:"identity"`
}

func PlanAuthenticationTransport(state, workspace, remote string, identity PolicyResourceIdentity) (*AuthenticationTransport, error) {
	if !filepath.IsAbs(state) || !filepath.IsAbs(workspace) || identity.ProjectID == "" || identity.ExecutionID == "" || identity.AttemptID == "" || identity.Generation == "" || identity.ConfigDigest == "" {
		return nil, errors.New("authentication_api_binding_invalid")
	}
	if _, err := githubTarget(remote); err != nil {
		return nil, err
	}
	p := &AuthenticationTransport{StateDirectory: filepath.Clean(state), Workspace: filepath.Clean(workspace), Remote: remote, Identity: identity}
	raw, _ := json.Marshal(p)
	digest := sha256.Sum256(raw)
	key := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:16]))
	p.Directory = filepath.Join(p.StateDirectory, "a", key)
	if len(filepath.Join(p.Directory, "s")) > authenticationSocketLimit {
		return nil, errors.New("authentication_api_socket_path_too_long")
	}
	return p, nil
}

func (p *AuthenticationTransport) validate() error {
	if p == nil {
		return errors.New("authentication_api_binding_missing")
	}
	expected, err := PlanAuthenticationTransport(p.StateDirectory, p.Workspace, p.Remote, p.Identity)
	if err != nil {
		return err
	}
	if *expected != *p {
		return errors.New("authentication_api_binding_mismatch")
	}
	return nil
}

// Only plain, canonical directories are admitted. In particular, a junction or
// symlink must not turn the narrow native grant into a grant for another tree.
func plainAuthenticationDirectory(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("authentication_api_directory_unverified")
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || !sameWorkspace(real, path) {
		return nil, errors.New("authentication_api_directory_redirected")
	}
	return info, nil
}

type authenticationSocketLease struct {
	plan      AuthenticationTransport
	root      *os.Root
	directory os.FileInfo
	socket    os.FileInfo
	cleaned   bool
	closed    bool
}

func acquireAuthenticationSocket(p *AuthenticationTransport) (*authenticationSocketLease, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if _, err := plainAuthenticationDirectory(p.StateDirectory); err != nil {
		return nil, err
	}
	parent := filepath.Dir(p.Directory)
	if err := os.Mkdir(parent, 0700); err != nil && !os.IsExist(err) {
		return nil, errors.New("authentication_api_directory_unavailable")
	}
	if _, err := plainAuthenticationDirectory(parent); err != nil {
		return nil, err
	}
	// Never adopt or delete an existing directory, including a stale original
	// lease after a crash. That original effect requires explicit reconciliation.
	if err := os.Mkdir(p.Directory, 0700); err != nil {
		return nil, errors.New("authentication_api_directory_conflict")
	}
	info, err := plainAuthenticationDirectory(p.Directory)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(p.Directory)
	if err != nil {
		_ = os.Remove(p.Directory) // exact, newly created empty directory only
		return nil, errors.New("authentication_api_directory_unavailable")
	}
	return &authenticationSocketLease{plan: *p, root: root, directory: info}, nil
}

func (l *authenticationSocketLease) verify() error {
	info, err := plainAuthenticationDirectory(l.plan.Directory)
	if err != nil || !os.SameFile(info, l.directory) {
		return errors.New("authentication_api_directory_identity_changed")
	}
	if l.socket != nil {
		info, err := l.root.Lstat("s")
		if err != nil || !os.SameFile(info, l.socket) {
			return errors.New("authentication_api_socket_identity_changed")
		}
	}
	return nil
}

func (l *authenticationSocketLease) close() bool {
	if l == nil {
		return true
	}
	if l.closed {
		return l.cleaned
	}
	l.closed = true
	// Remove only the exact socket inode we created, relative to the open root.
	// No recursive removal and no deletion of foreign files or replacement roots.
	info, err := plainAuthenticationDirectory(l.plan.Directory)
	ok := err == nil && os.SameFile(info, l.directory)
	if l.socket != nil {
		info, err := l.root.Lstat("s")
		if os.IsNotExist(err) {
			// A closed listener may already have removed its socket.
		} else if err != nil || !os.SameFile(info, l.socket) {
			ok = false
		} else if err := l.root.Remove("s"); err != nil {
			ok = false
		}
	}
	if err := l.root.Close(); err != nil {
		ok = false
	}
	info, err = plainAuthenticationDirectory(l.plan.Directory)
	if ok && err == nil && os.SameFile(info, l.directory) {
		ok = os.Remove(l.plan.Directory) == nil
	} else {
		ok = false
	}
	l.cleaned = ok
	return ok
}

// The four Git roots retain their original meaning. Authentication gets one
// additional per-execution grant, never the shared a/ parent or Runner state.
func (b Backend) toolWriteArgs(linked []string) []string {
	args := append([]string(nil), linked...)
	if p := b.Config.AuthenticationTransport; p != nil {
		args = append(args, "--add-dir", p.Directory)
	}
	return args
}

func (l *ExecutionLifecycle) bindAuthenticationTransport(snapshot executionbackend.LifecycleSnapshot) error {
	auth := l.Backend.Config.Authentication
	if auth == nil {
		return nil
	}
	s := snapshot.Session
	identity := PolicyResourceIdentity{ProjectID: s.ProjectID, ExecutionID: s.ExecutionID, AttemptID: s.AttemptID, Generation: snapshot.ResourceGeneration, ConfigDigest: snapshot.ConfigurationDigest}
	if snapshot.PolicyResources == nil {
		return errors.New("authentication_api_binding_missing")
	}
	payload, err := DecodePolicyResourceEnvelope(*snapshot.PolicyResources, identity)
	if err != nil {
		return err
	}
	expected, err := PlanAuthenticationTransport(l.Backend.Config.RunDir, snapshot.Ref.WorkspacePath, auth.remoteURL, identity)
	if err != nil {
		return err
	}
	if payload.AuthenticationTransport == nil || *payload.AuthenticationTransport != *expected {
		return errors.New("authentication_api_binding_mismatch")
	}
	if original := l.Backend.Config.AuthenticationTransport; original != nil && *original != *expected {
		return errors.New("authentication_api_binding_mismatch")
	}
	l.Backend.Config.AuthenticationTransport = expected
	return nil
}
