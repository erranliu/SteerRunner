package codexworker

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestExecutionOwnerCheckpointPinsOriginalTerminalAndSignature(t *testing.T) {
	const originalCommand = "0123456789abcdef0123456789abcdef"
	r := ownerTestRequest(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner := ExecutionOwnerIdentity{Protocol: ExecutionOwnerProtocol, Directory: t.TempDir(), Process: r.Snapshot.Ref.External, RuntimeSHA256: string(make([]byte, 64)), ExitPublicKey: hex.EncodeToString(public), WorkspaceCheckpoints: true}
	binding, err := NewAppServerBindingEnvelope(AppServerBindingIdentity{ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, ConfigDigest: r.Snapshot.ConfigurationDigest}, AppServerBinding{Endpoint: "ws://127.0.0.1:12345", Generation: "original", Owner: &owner})
	if err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Implementation = []b.ImplementationDataEnvelope{binding}
	checkpoint := ownerWorkspaceCheckpoint{Owner: owner, Binding: *r.Operation.LifecycleIdentity, Workspace: r.Snapshot.WorkspacePlan, CommandID: originalCommand, SessionID: "session", TurnID: "turn", Fact: b.WorkspaceInspection{Qualified: true, Head: "original-head", Tree: "original-tree"}}
	body, _ := json.Marshal(checkpoint)
	path, err := checkpointPath(owner, originalCommand)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, encode(signedRemoteExit{Body: data, Signature: ed25519.Sign(private, body)}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(body)
	fact, _, err := ReadExecutionCheckpoint(r.Snapshot, originalCommand, "session", "turn")
	if err != nil || !fact.Qualified || fact.Head != "original-head" {
		t.Fatal("original checkpoint unavailable", err)
	}
	for _, field := range []string{"command", "session", "turn", "execution", "workspace"} {
		s := r.Snapshot.Clone()
		command, session, turn := originalCommand, "session", "turn"
		switch field {
		case "command":
			command = "replacement"
		case "session":
			session = "replacement"
		case "turn":
			turn = "replacement"
		case "execution":
			s.Session.ExecutionID = "replacement"
		case "workspace":
			s.WorkspacePlan.Path = "replacement"
		}
		if _, _, err := ReadExecutionCheckpoint(s, command, session, turn); err == nil {
			t.Fatalf("accepted different %s", field)
		}
	}
	checkpoint.Fact.Head = "replacement-head"
	changed, _ := json.Marshal(checkpoint)
	write(changed)
	if _, _, err := ReadExecutionCheckpoint(r.Snapshot, originalCommand, "session", "turn"); err == nil {
		t.Fatal("tampered checkpoint accepted")
	}
}
