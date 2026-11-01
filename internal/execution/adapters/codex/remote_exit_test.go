package codexworker

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRemoteStopBindingNeedsNoExitArtifact(t *testing.T) {
	workspace := t.TempDir()
	// Even a blocked legacy export root cannot affect pure binding generation.
	if err := os.WriteFile(filepath.Join(workspace, ".sr-runtime-stage"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	public, err := newRemoteStopBinding()
	other, otherErr := newRemoteStopBinding()
	if err != nil || otherErr != nil || len(public) != ed25519.PublicKeySize || bytes.Equal(public, other) {
		t.Fatal("invalid or reused stop binding", err, otherErr)
	}
	ready := RemoteReadiness{ExitPublicKey: public, Workspace: workspace, SessionID: "original"}
	stop, err := RemoteStopPath(ready)
	if err != nil || stop != ".sr-runtime-stage/native-stop/"+hex.EncodeToString(public)+"/request.json" {
		t.Fatal("changed request address", stop, err)
	}
}

func TestRemoteStopRequiresCompleteOriginalReadiness(t *testing.T) {
	public, err := newRemoteStopBinding()
	if err != nil {
		t.Fatal(err)
	}
	ready := RemoteReadiness{ExitPublicKey: public, Workspace: t.TempDir(), SessionID: "original", ConfigurationDigest: "original-config", CheckedAt: time.Now().UTC()}
	relative, _ := RemoteStopPath(ready)
	name := filepath.Join(ready.Workspace, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"foreign", "partial", "oversized", "original"} {
		t.Run(mode, func(t *testing.T) {
			request := ready
			if mode == "foreign" {
				request.ConfigurationDigest = "replacement"
			}
			raw := encode(request)
			if mode == "partial" {
				raw = []byte(`{"session_id":"original"}`)
			}
			if mode == "oversized" {
				raw = append(raw, bytes.Repeat([]byte(" "), 8193)...)
			}
			if err := os.WriteFile(name, raw, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // first read must decide; invalid requests cannot outlive the context
			if got := awaitRemoteStop(ctx, ready); got != (mode == "original") {
				t.Fatal(mode, got)
			}
		})
	}
	exit, _ := RemoteExitPath(ready)
	if _, err := os.Stat(filepath.Join(ready.Workspace, filepath.FromSlash(exit))); !os.IsNotExist(err) {
		t.Fatal("unexpected exit artifact", err)
	}
}

func TestRemoteExitAccountsForOriginalOwnedTree(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Orca Codex is a Windows execution-host contract")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestRemoteExitSleepingChild$")
	command.Env = append(os.Environ(), "SR_EXIT_TEST_CHILD=1")
	owned, err := process.New(time.Second).StartManaged(command)
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	defer command.Wait()
	if process.Probe(ptrIdentity(owned.Identity())).State != "matches" {
		t.Fatal("original child is not live")
	}
	if !settleRemoteTree(owned) {
		t.Fatal("original Job tree did not settle")
	}
	if process.Probe(ptrIdentity(owned.Identity())).State != "absent" {
		t.Fatal("original child survived settlement")
	}
}

func ptrIdentity(identity b.ProcessIdentity) *b.ProcessIdentity { return &identity }

func TestRemoteExitSleepingChild(t *testing.T) {
	if os.Getenv("SR_EXIT_TEST_CHILD") != "1" {
		return
	}
	time.Sleep(time.Minute)
}

func TestRemoteReaderFailedStartStillProvesNoNativeReader(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable failure contract")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	workspace := t.TempDir()
	invalid := filepath.Join(t.TempDir(), "codex.exe")
	// PE magic, but not a loadable image: no native reader can be created.
	if err := os.WriteFile(invalid, []byte("MZ invalid executable"), 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := process.CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := RuntimeBinarySHA256()
	if err != nil {
		t.Fatal(err)
	}
	config := RemoteAgentConfig{Executable: invalid, RepositoryURL: "https://github.com/erranliu/SteerlineTest.git"}
	ready := RemoteReadiness{Verified: true, NativeTransport: RemoteNativeTransport, Workspace: workspace, SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", RuntimeSHA256: hash, ConfigurationDigest: RemoteConfigurationDigest(config), Repository: config.RepositoryURL, Process: *identity}
	dir := RemoteRuntimeDirectory(workspace)
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "ready.json"), encode(ready), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "configuration.json"), encode(config), 0600); err != nil {
		t.Fatal(err)
	}
	query := RemoteTurnQuery{Workspace: workspace, Session: ready.SessionID, InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	value, err := InspectRemoteTurn(context.Background(), query)
	if err == nil || !value.InspectionFailed || !value.ReaderWritersAbsent || value.Started || value.Completed || value.Readiness.Process != *identity {
		t.Fatal("failed creation lost its positive no-reader proof", value, err)
	}
}
