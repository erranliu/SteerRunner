package codexworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestRemoteOwnershipPreparationFailureStillSignsSettlement(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	// No native executable can be launched. Ownership must precede preparation.
	err := RunRemoteAgent(context.Background(), RemoteAgentConfig{Executable: filepath.Join(t.TempDir(), "missing.exe"), RepositoryURL: "https://github.com/erranliu/SteerlineTest.git"}, strings.NewReader(""), &out, io.Discard)
	if err == nil {
		t.Fatal("invalid native unexpectedly prepared")
	}
	var owner RemoteOwnership
	line := strings.TrimSpace(out.String())
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, RemoteOwnerPrefix))
	if e != nil || json.Unmarshal(raw, &owner) != nil || owner.Owner.PID <= 0 || len(owner.ExitPublicKey) != 32 {
		t.Fatal("failure lost original helper ownership", out.String(), e)
	}
	relative, _ := RemoteExitPath(RemoteReadiness{ExitPublicKey: owner.ExitPublicKey})
	exit, e := os.ReadFile(filepath.Join(owner.Workspace, filepath.FromSlash(relative)))
	if e != nil || VerifyRemoteOwnershipExit(exit, owner) != nil {
		t.Fatal("preparation failure lost original signed cleanup", e)
	}
}

func TestRemoteOwnershipRequiresSignedCompleteOriginalExit(t *testing.T) {
	for _, mode := range []string{"complete", "writers", "authentication", "owner", "key", "config", "timestamp", "tamper"} {
		t.Run(mode, func(t *testing.T) {
			writer, key, err := newRemoteExitWriter(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer writer.close()
			owner := RemoteOwnership{ExitPublicKey: key, Owner: b.ProcessIdentity{Kind: "os", PID: 1, Created: "original"}, ConfigurationDigest: "original", CreatedAt: time.Now().UTC()}
			proof := RemoteExit{Ownership: &owner, WritersAbsent: true, AuthenticationClosed: true, ClosedAt: time.Now().UTC()}
			if mode == "writers" {
				proof.WritersAbsent = false
			}
			if mode == "authentication" {
				proof.AuthenticationClosed = false
			}
			if mode == "timestamp" {
				proof.ClosedAt = owner.CreatedAt.Add(-time.Second)
			}
			if err = writer.write(proof); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(writer.file.Name())
			if err != nil {
				t.Fatal(err)
			}
			if mode == "owner" {
				owner.Owner.Created = "replacement"
			}
			if mode == "config" {
				owner.ConfigurationDigest = "replacement"
			}
			if mode == "key" {
				owner.ExitPublicKey = make([]byte, 32)
			}
			if mode == "tamper" {
				raw = bytes.Replace(raw, []byte("original"), []byte("replaced"), 1)
			}
			if (VerifyRemoteOwnershipExit(raw, owner) == nil) != (mode == "complete") {
				t.Fatal("invalid ownership settlement accepted", mode)
			}
		})
	}
}

func TestRemoteOwnershipStopCancelsBeforeReadiness(t *testing.T) {
	writer, key, err := newRemoteExitWriter(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.close()
	owner := RemoteOwnership{ExitPublicKey: key, Workspace: t.TempDir(), ConfigurationDigest: "original"}
	path, _ := RemoteOwnershipStopPath(owner)
	name := filepath.Join(owner.Workspace, filepath.FromSlash(path))
	if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name, encode(owner), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !awaitRemoteOwnershipStop(ctx, owner) {
		t.Fatal("initialization stop required business readiness")
	}
}

type failedRemoteHandle struct{}

func (*failedRemoteHandle) Kill()                       { panic("typed nil must not be called") }
func (*failedRemoteHandle) Close()                      { panic("typed nil must not be called") }
func (*failedRemoteHandle) Identity() b.ProcessIdentity { panic("typed nil must not be called") }
func TestRemoteOwnershipFailedLaunchKeepsUnknownCreationUnsettled(t *testing.T) {
	for _, created := range []bool{false, true} {
		cmd := exec.Command("unused")
		if created {
			cmd.Process = &os.Process{Pid: 1}
		}
		handle, actual, err := startRemoteTree(func(*exec.Cmd) (b.StopHandle, error) {
			var absent *failedRemoteHandle
			return absent, errors.New("startup failed")
		}, cmd)
		if err == nil || handle != nil || actual != created {
			t.Fatal("failed launch lost its original creation fact", handle, actual, err)
		}
		settled := settleRemoteTree(handle) && (!actual || handle != nil)
		if settled == created {
			t.Fatal("unknown native creation became settled", created, settled)
		}
	}
}
