package codexworker

import (
	"context"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteAuthenticationStopUsesOnlyOriginalProtectedOwner(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LOCALAPPDATA", state)
	work := filepath.Join(state, "work")
	dir := RemoteRuntimeDirectory(work)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := process.CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	sha, err := RuntimeBinarySHA256()
	if err != nil {
		t.Fatal(err)
	}
	cfg := RemoteAgentConfig{RepositoryURL: "https://github.com/owner/repo.git", Executable: "codex.exe"}
	ready := RemoteReadiness{Verified: true, NativeTransport: RemoteNativeTransport, Contract: PolicyContract, Repository: cfg.RepositoryURL, AuthenticationControl: true, Workspace: work, SessionID: "01a1453f-0327-7f21-b0b9-ee79dad66108", ConfigurationDigest: RemoteConfigurationDigest(cfg), RuntimeSHA256: sha, Process: *identity}
	write := func(name string, v any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// A retired null field must not be discarded while recomputing a digest.
	rawConfig, _ := json.Marshal(cfg)
	var legacy map[string]any
	_ = json.Unmarshal(rawConfig, &legacy)
	legacy["github_auth"] = nil
	write("configuration.json", legacy)
	write("ready.json", ready)
	if _, err := RevokeRemoteAuthentication(context.Background(), RemoteAuthenticationQuery{Workspace: work, Session: ready.SessionID}); err == nil {
		t.Fatal("legacy runtime configuration adopted as gh")
	}
	if _, err := os.Stat(filepath.Join(dir, "authentication-stop.json")); !os.IsNotExist(err) {
		t.Fatal("legacy runtime received a stop request under a changed configuration")
	}
	write("configuration.json", cfg)
	write("ready.json", ready)
	a := testAuthentication(t)
	var revoked atomic.Int32
	a.onClose = func(ok bool) {
		revoked.Add(1)
		write("authentication-cleanup.json", map[string]any{"session_id": ready.SessionID, "configuration_digest": ready.ConfigurationDigest, "revocation_confirmed": ok, "broker": map[string]any{"closed": ok, "socket_cleanup_confirmed": ok, "mode": "host-gh-repository-broker"}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go watchRemoteAuthentication(ctx, dir, ready, a)
	if _, err := RevokeRemoteAuthentication(ctx, RemoteAuthenticationQuery{Workspace: work, Session: "foreign"}); err == nil {
		t.Fatal("foreign session stopped original authentication")
	}
	if revoked.Load() != 0 {
		t.Fatal("foreign request reached credential owner")
	}
	proof, err := RevokeRemoteAuthentication(ctx, RemoteAuthenticationQuery{Workspace: work, Session: ready.SessionID})
	if err != nil || !proof.Revoked || revoked.Load() != 1 {
		t.Fatal("original revocation missing", err, revoked.Load())
	}
	if _, err = RevokeRemoteAuthentication(ctx, RemoteAuthenticationQuery{Workspace: work, Session: ready.SessionID}); err != nil || revoked.Load() != 1 {
		t.Fatal("revocation replay repeated its effect", err, revoked.Load())
	}
	write("authentication-cleanup.json", map[string]any{"session_id": ready.SessionID, "configuration_digest": ready.ConfigurationDigest, "revocation_confirmed": true, "broker": map[string]any{"closed": true, "socket_cleanup_confirmed": true, "mode": "github-app-repository-broker"}})
	if _, err := readRemoteAuthenticationCleanup(dir, ready); err == nil {
		t.Fatal("retired App cleanup proof admitted as gh")
	}
}
