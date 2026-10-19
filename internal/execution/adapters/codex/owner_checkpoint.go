package codexworker

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

// This is a terminal observation, not another workflow checkpoint or journal.
// It is written before the original connection publishes turn/completed.
type ownerWorkspaceCheckpoint struct {
	Owner                        ExecutionOwnerIdentity
	Binding                      b.LifecycleOperationIdentity
	CommandID, SessionID, TurnID string
	Workspace                    b.WorkspaceIntent
	Fact                         b.WorkspaceInspection
	Diagnostic                   string
}

func checkpointPath(identity ExecutionOwnerIdentity, commandID string) (string, error) {
	if !b.StableExecutionID(commandID) {
		return "", errors.New("invalid original checkpoint command")
	}
	return filepath.Join(identity.Directory, "checkpoints", commandID+".json"), nil
}

func writeOwnerCheckpoint(setup b.LifecycleSetup, identity ExecutionOwnerIdentity, key ed25519.PrivateKey, request b.LifecycleRequest, commandID, sessionID, turnID string) error {
	if request.Input.Turn == nil || request.Input.Turn.CommandID != commandID || request.Operation.LifecycleIdentity == nil {
		return errors.New("terminal checkpoint lacks original input")
	}
	path, err := checkpointPath(identity, commandID)
	if err != nil {
		return err
	}
	// Repeated notifications cannot replace the original completion observation.
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	git := workspace.Git{Command: b.LaunchCommand{Executable: setup.GitExecutable}, Timeout: 10 * time.Second, Start: setup.ProcessHost.StartManaged}
	repo, inspectErr := workspace.ResolveRepository(ctx, git, setup.RepositoryPath)
	fact := b.WorkspaceInspection{}
	if inspectErr == nil {
		fact, inspectErr = workspace.InspectAudit(ctx, git, repo, &request.Snapshot.WorkspacePlan, "")
	}
	checkpoint := ownerWorkspaceCheckpoint{Owner: identity, Binding: *request.Operation.LifecycleIdentity, CommandID: commandID, SessionID: sessionID, TurnID: turnID, Workspace: request.Snapshot.WorkspacePlan, Fact: fact}
	if inspectErr != nil {
		checkpoint.Diagnostic = "original completion workspace inspection failed"
	}
	body, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return setup.ProcessHost.WriteExecutionFile(path, encode(signedRemoteExit{Body: body, Signature: ed25519.Sign(key, body)}))
}

// ReadExecutionCheckpoint verifies only the original signed terminal facts.
// Missing/invalid facts never authorize reconstructing them from current HEAD.
func ReadExecutionCheckpoint(snapshot b.LifecycleSnapshot, commandID, sessionID, turnID string) (b.WorkspaceInspection, string, error) {
	identity := ExecutionOwner(snapshot)
	if identity == nil || !identity.WorkspaceCheckpoints {
		return b.WorkspaceInspection{}, "", errors.New("original owner has no completion checkpoint contract")
	}
	path, err := checkpointPath(*identity, commandID)
	if err != nil {
		return b.WorkspaceInspection{}, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return b.WorkspaceInspection{}, "", err
	}
	var signed signedRemoteExit
	var checkpoint ownerWorkspaceCheckpoint
	key, keyErr := hex.DecodeString(identity.ExitPublicKey)
	if len(raw) > 1<<20 || keyErr != nil || len(key) != ed25519.PublicKeySize || b.DecodeStrictJSON(raw, &signed, "invalid terminal checkpoint") != nil || !ed25519.Verify(key, signed.Body, signed.Signature) || b.DecodeStrictJSON(signed.Body, &checkpoint, "invalid terminal checkpoint body") != nil {
		return b.WorkspaceInspection{}, "", errors.New("original terminal checkpoint signature is unverified")
	}
	id := checkpoint.Binding
	if checkpoint.Owner != *identity || checkpoint.CommandID != commandID || checkpoint.SessionID != sessionID || checkpoint.TurnID != turnID || checkpoint.Workspace != snapshot.WorkspacePlan || id.Ref != snapshot.Ref || id.ProjectID != snapshot.Session.ProjectID || id.ExecutionID != snapshot.Session.ExecutionID || id.ConfigurationDigest != snapshot.ConfigurationDigest || id.ResourceGeneration != snapshot.ResourceGeneration {
		return b.WorkspaceInspection{}, "", errors.New("original terminal checkpoint identity changed")
	}
	return checkpoint.Fact, checkpoint.Diagnostic, nil
}
