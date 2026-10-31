package codexworker

import (
	"context"
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRemoteExitRequiresOriginalSignedCompleteSettlement(t *testing.T) {
	for _, mode := range []string{"complete", "live-tree", "changed-body", "other-run", "other-key", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			workspace := t.TempDir()
			writer, public, err := newRemoteExitWriter(workspace)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.close()
			ready := RemoteReadiness{ExitPublicKey: public, SessionID: "original-session", ConfigurationDigest: "original-config", Owner: b.ProcessIdentity{PID: 1, Created: "original-helper"}, CheckedAt: time.Now().UTC()}
			exit := RemoteExit{Readiness: ready, WritersAbsent: true, ClosedAt: time.Now().UTC()}
			if mode == "live-tree" {
				exit.WritersAbsent = false
			}

			if err = writer.write(exit); err != nil {
				t.Fatal(err)
			}
			relative, _ := RemoteExitPath(ready)
			raw, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(relative)))
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "changed-body":
				var envelope signedRemoteExit
				if json.Unmarshal(raw, &envelope) != nil {
					t.Fatal("invalid fixture")
				}
				exit.Readiness.Owner.Created = "different-helper"
				envelope.Body = encode(exit)
				raw = encode(envelope)
			case "other-run":
				ready.SessionID = "replacement-session"
			case "other-key":
				ready.ExitPublicKey = make([]byte, 32)
			case "truncated":
				raw = raw[:len(raw)-10]
			}
			if err = VerifyRemoteExit(raw, ready); (err == nil) != (mode == "complete") {
				t.Fatal(mode, err)
			}
		})
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
