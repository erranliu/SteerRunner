package application

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func bindNativeProofIdentity(r *runstate.Record) {
	r.InputDigest = "original-config"
	r.Snapshot.ConfigurationDigest = r.InputDigest
	r.Snapshot.ResourceGeneration = "original-generation"
	r.Snapshot.Session.ProjectID = "original-project"
	r.Snapshot.Session.ExecutionID = r.ID
	r.Snapshot.Session.WorkflowRunID = r.ID
	r.Snapshot.Ref.RunID = r.ID
	r.Snapshot.Ref.AttemptID = r.Snapshot.Session.AttemptID
}

// The fixture encodes the existing signed wire contract, without changing the
// shared auditEngine, native owner fixture, or their original assertions.
func nativeProofFixture(t *testing.T, e *Engine, w *worker) (workflow.StageInput, func(b.WorkspaceInspection, string), string) {
	t.Helper()
	r := e.runs["run"]
	bindNativeProofIdentity(r)
	input := *workflow.Input(r.Workflow, r.Snapshot.PendingTurn.CommandID)
	r.Snapshot.Session.CurrentTurnID = "original-turn"
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner := codex.ExecutionOwnerIdentity{WorkspaceCheckpoints: true, Protocol: codex.ExecutionOwnerProtocol, Directory: t.TempDir(), Process: b.ProcessIdentity{Kind: "os", PID: 1, Created: "original", Executable: "runner.exe"}, RuntimeSHA256: strings.Repeat("a", 64), ExitPublicKey: hex.EncodeToString(public)}
	envelope, err := codex.NewAppServerBindingEnvelope(codex.AppServerBindingIdentity{ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, ConfigDigest: r.Snapshot.ConfigurationDigest}, codex.AppServerBinding{Owner: &owner, Endpoint: "ws://127.0.0.1:12345", Generation: "original"})
	if err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Implementation = []b.ImplementationDataEnvelope{envelope}
	path := filepath.Join(owner.Directory, "checkpoints", input.CommandID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(fact b.WorkspaceInspection, turn string) {
		t.Helper()
		// Exact existing ownerWorkspaceCheckpoint and signedRemoteExit JSON.
		body, err := json.Marshal(struct {
			Owner                        codex.ExecutionOwnerIdentity
			Binding                      b.LifecycleOperationIdentity
			CommandID, SessionID, TurnID string
			Workspace                    b.WorkspaceIntent
			Fact                         b.WorkspaceInspection
			Diagnostic                   string
		}{owner, b.LifecycleOperationIdentity{Ref: r.Snapshot.Ref, ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, ConfigurationDigest: r.Snapshot.ConfigurationDigest, ResourceGeneration: r.Snapshot.ResourceGeneration}, input.CommandID, r.Snapshot.Session.SessionID, turn, r.Snapshot.WorkspacePlan, fact, "original diagnostic"})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(struct {
			Body      json.RawMessage `json:"body"`
			Signature []byte          `json:"signature"`
		}{body, ed25519.Sign(private, body)})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// A tempting cache must never stand in for the required original proof.
	w.checkpoints = map[string]*workflow.WorkspaceSource{input.CommandID: {Qualified: true, Head: "cached-replacement"}}
	w.ownedInputs = map[string]bool{input.CommandID: true}
	return input, write, path
}

func TestNativeCompletionProofWaitsForOriginalDespiteCacheAndReload(t *testing.T) {
	e, w := auditEngine(t)
	input, write, path := nativeProofFixture(t, e, w)
	r := e.runs["run"]
	if got := e.originalCheckpoint(r, w, input); got != nil {
		t.Fatal("missing required proof used cache/current HEAD", got)
	}
	if err := e.commitLocked(r.Clone(), nil, "original_binding"); err != nil {
		t.Fatal(err)
	}
	runs, err := e.store.Runs()
	if err != nil || len(runs) != 1 {
		t.Fatal(err)
	}
	r = runs[0]
	fact := b.WorkspaceInspection{Repository: r.RepositoryPath, Worktree: r.Snapshot.WorkspacePlan.Path, Branch: r.Snapshot.WorkspacePlan.Branch, Base: r.Snapshot.WorkspacePlan.BaseCommit, Head: "original-head", Tree: "original-tree", Qualified: true}
	write(fact, "wrong-turn")
	if got := e.originalCheckpoint(r, w, input); got != nil {
		t.Fatal("foreign signed turn used cache", got)
	}
	write(fact, r.Snapshot.Session.CurrentTurnID)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte("original-head"), []byte("tampered-head"), 1), 0600); err != nil {
		t.Fatal(err)
	}
	if got := e.originalCheckpoint(r, w, input); got != nil {
		t.Fatal("tampered proof used cache", got)
	}
	write(fact, r.Snapshot.Session.CurrentTurnID)
	got := e.originalCheckpoint(r, w, input)
	if got == nil || !got.Qualified || got.Head != fact.Head || got.Diagnostic != "original diagnostic" {
		t.Fatal("late original proof not accepted", got)
	}
	fact.Repository = "different-repository"
	write(fact, r.Snapshot.Session.CurrentTurnID)
	if got := e.originalCheckpoint(r, w, input); got == nil || got.Qualified || !strings.Contains(got.Diagnostic, "frozen repository") {
		t.Fatal("business repository check removed", got)
	}
	// An original owner without this contract keeps the same fallback route.
	var binding codex.AppServerBinding
	if err := json.Unmarshal(r.Snapshot.Implementation[0].Payload, &binding); err != nil {
		t.Fatal(err)
	}
	binding.Owner.WorkspaceCheckpoints = false
	envelope, err := codex.NewAppServerBindingEnvelope(codex.AppServerBindingIdentity{ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, ConfigDigest: r.Snapshot.ConfigurationDigest}, binding)
	if err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Implementation = []b.ImplementationDataEnvelope{envelope}
	if got := e.originalCheckpoint(r, w, input); got == nil || got.Head != "cached-replacement" {
		t.Fatal("non-checkpoint owner fallback changed", got)
	}
	// A binding without an owner contract keeps the existing fallback cache too.
	r.Snapshot.Implementation = nil
	if got := e.originalCheckpoint(r, w, input); got == nil || got.Head != "cached-replacement" {
		t.Fatal("non-contract cache path changed", got)
	}
}

func TestNativeCompletionProofKeepsAuditAncestryCheck(t *testing.T) {
	e, w := auditEngine(t)
	finishStage(t, e, "实施及必要验证已完成")
	r := e.runs["run"]
	pre := checkpoint(r, r.Workflow.ImplementationResultID)
	input, write, _ := nativeProofFixture(t, e, w)
	gitTest(t, pre.Worktree, "checkout", "--orphan", "unrelated")
	gitTest(t, pre.Worktree, "commit", "--allow-empty", "-m", "unrelated")
	head := gitTest(t, pre.Worktree, "rev-parse", "HEAD")
	fact := b.WorkspaceInspection{Repository: r.RepositoryPath, Worktree: r.Snapshot.WorkspacePlan.Path, Branch: r.Snapshot.WorkspacePlan.Branch, Base: r.Snapshot.WorkspacePlan.BaseCommit, Head: head, Tree: pre.Tree, Qualified: true}
	write(fact, r.Snapshot.Session.CurrentTurnID)
	if got := e.originalCheckpoint(r, w, input); got == nil || got.Qualified || !strings.Contains(got.Diagnostic, "not a descendant") {
		t.Fatal("native proof bypassed audit ancestry", got)
	}
}

func TestNativeCompletionProofWaitsAtResultCommitAndPreservesStop(t *testing.T) {
	for _, stop := range []bool{false, true} {
		name := "original-continuation"
		if stop {
			name = "stop-before-proof"
		}
		t.Run(name, func(t *testing.T) {
			e, w := auditEngine(t)
			input, write, _ := nativeProofFixture(t, e, w)
			r := e.runs["run"]
			r.Snapshot.Session.CurrentCommandID = input.CommandID
			inputs, intents := len(r.Workflow.StageInputs), len(r.Intents)
			value := b.TurnEvidence{CommandID: input.CommandID, SessionID: r.Snapshot.Session.SessionID, TurnID: r.Snapshot.Session.CurrentTurnID, Started: true, Completed: true, Succeeded: true, FinalVerified: true, FinalSource: "native", FinalItemID: "original-item", LastMessage: "实施及必要验证已完成"}
			if e.finish(r.ID, value) {
				t.Fatal("missing proof published a successor")
			}
			waiting := e.runs[r.ID]
			if waiting.Phase != "completion_checkpoint_unavailable" || len(waiting.Workflow.StageInputs) != inputs || len(waiting.Workflow.StageResults) != 0 || len(waiting.Workflow.Results) != 0 || len(waiting.Intents) != intents {
				t.Fatal("missing proof consumed the original result or created work", waiting.Phase)
			}
			if stop {
				next := waiting.Clone()
				next.Allowed, next.ResumeRequired = false, true
				next.PendingStop, next.StopStatus = "original-stop", "pending"
				next.CommandStates[next.PendingStop] = "pending"
				if err := e.commitLocked(next, nil, "stop_requested"); err != nil {
					t.Fatal(err)
				}
			}
			fact := b.WorkspaceInspection{Repository: r.RepositoryPath, Worktree: r.Snapshot.WorkspacePlan.Path, Branch: r.Snapshot.WorkspacePlan.Branch, Base: r.Snapshot.WorkspacePlan.BaseCommit, Head: gitTest(t, r.RepositoryPath, "rev-parse", "HEAD"), Tree: gitTest(t, r.RepositoryPath, "rev-parse", "HEAD^{tree}"), Qualified: true}
			write(fact, value.TurnID)
			e.finish(r.ID, value)
			got := e.runs[r.ID]
			result := workflow.Result(got.Workflow, input.CommandID)
			if result == nil || result.Workspace == nil || result.Workspace.Head != fact.Head || len(got.Intents) != intents {
				t.Fatal("late original proof lost its source or created a native operation")
			}
			if stop {
				if got.Allowed || !got.ResumeRequired || got.PendingStop != "original-stop" || got.StopStatus != "pending" || len(got.Workflow.StageInputs) != inputs || got.NextStage != nil || got.PendingPRURL != "" {
					t.Fatal("late original proof bypassed the committed stop")
				}
			} else if len(got.Workflow.StageInputs) != inputs+1 || got.Workflow.StepID != workflow.StepAuditCode {
				t.Fatal("original continuation did not resume after the proof arrived")
			}
		})
	}
}

func TestNativePolicyMismatchBlocksNewDispatch(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"]
	bindNativeProofIdentity(r)
	r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
	permissions, err := codex.NativePermissions("windows", "local", "codex", "read_only")
	if err != nil {
		t.Fatal(err)
	}
	identity := codex.PolicyResourceIdentity{ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, ConfigDigest: r.Snapshot.ConfigurationDigest}
	resource, err := codex.NewPolicyResourceEnvelope(identity, codex.PolicyResourcePayload{NativeVersion: codex.PolicyCodexVersion, Contract: permissions.Contract, Settings: b.LaunchCommand{Executable: "original.exe", Args: permissions.Args()}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := codex.EncodeProfileConfig(codex.ProfileConfig{AccessMode: "full_access", Executable: "original.exe"})
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: codex.ProfileCodecKey, CodecVersion: codex.ProfileCodecVersion, Purpose: b.ImplementationConfigPurpose, ProjectID: identity.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}, raw)
	if err != nil {
		t.Fatal(err)
	}
	r.Snapshot.PolicyResources, r.Profile = &resource, []b.ImplementationDataEnvelope{profile}
	before := len(r.Intents)
	if e.advance(r.ID, w) {
		t.Fatal("mismatched permission resource advanced")
	}
	got := e.runs[r.ID]
	if got.Phase != "permission_resources_unverified" || got.Allowed || !got.ResumeRequired || len(got.Intents) != before {
		t.Fatal("new dispatch gate changed", got.Phase, got.Diagnostic)
	}
}

func TestNativeProofBusinessKnowledgeBoundary(t *testing.T) {
	for file, forbidden := range map[string][]string{
		"policy.go":           {"sandbox_mode=", "profile.AccessMode", "DecodeProfileConfigEnvelope"},
		"workspace_source.go": {"/adapters/codex", "ExecutionOwner(", "WorkspaceCheckpoints", "ReadExecutionCheckpoint("},
	} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range forbidden {
			if strings.Contains(string(raw), private) {
				t.Fatalf("%s interprets private native knowledge %q", file, private)
			}
		}
	}
}
