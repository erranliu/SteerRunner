package application

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func retainedFixture(t *testing.T) (*baseRefreshFixture, *runstate.Record) {
	f := newBaseRefreshFixture(t, "origin", "main")
	f.request.Workflow = contracts.Workflow{ID: workflow.SingleExecution, Version: 1, PromptOverrides: map[string]string{"execute": "finish retained work"}}
	f.request.Publication = contracts.Publication{}
	f.input.Input = canonical(f.request)
	r, err := freeze(context.Background(), f.state, f.input)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := workspace.Prepare(context.Background(), gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, filepath.Dir(r.Snapshot.WorkspacePlan.Path), &r.Snapshot.WorkspacePlan)
	if err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
	r.WorkspaceFact = prepared.Inspection
	captureManagedWorkspace(context.Background(), r)
	if r.ManagedWorkspace == nil {
		t.Fatal("creation failed to bind incarnation")
	}
	r.Allowed, r.Closed, r.StopStatus, r.Status = false, true, "stopped", "stopped"
	return f, r
}

func TestWorktreeReuseColdStopBeforeFirstInputAllowsNextRun(t *testing.T) {
	f, original := retainedFixture(t)
	e, _, _ := operationEngine(t)
	e.dir = f.state
	if err := e.commitLocked(original, nil); err != nil {
		t.Fatal(err)
	}
	e.workers[original.ID] = &worker{wake: make(chan struct{}, 1)}
	f.request.ReuseWorktreeFromRunID = original.ID
	f.input.RequestID = "cold-reuse-submit"
	f.input.Input = canonical(f.request)
	next, err := freezeWithWorkspace(context.Background(), f.state, f.input, original)
	if err != nil {
		t.Fatal(err)
	}
	command := storage.NewCommand("submit", next.ID, f.input)
	next.SubmitCommandID = command.ID
	next.CommandStates[command.ID] = "pending"
	if err := e.commitLocked(next, &command); err != nil {
		t.Fatal(err)
	}
	w := &worker{wake: make(chan struct{}, 1), port: &operationCommitPort{}}
	e.workers[next.ID] = w
	stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "cold-reuse-stop", Input: canonical(contracts.RevisionInput{ExpectedRevision: next.Revision})}
	if response := e.Handle("stop", next.ID, "", "", "", 0, &stop); !response.OK {
		t.Fatal(response)
	}
	e.advance(next.ID, w)
	stopped := e.runs[next.ID]
	if stopped.StopStatus != "stopped" || stopped.CommandStates[command.ID] != "failed" || len(stopped.Workflow.StageInputs) != 0 || len(stopped.Intents) != 0 {
		t.Fatal("unsent submit stranded the retained workspace", stopped)
	}
	source, unlock, err := e.reusableSource(next.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	f.request.ReuseWorktreeFromRunID = next.ID
	f.input.Input = canonical(f.request)
	third, err := freezeWithWorkspace(context.Background(), f.state, f.input, source)
	if err != nil || third.Snapshot.Workspace != original.Snapshot.Workspace {
		t.Fatal("third Run cannot reuse cold-stopped successor", err)
	}
}

func TestWorktreeReuseRejectsChangedOrcaTransportBeforeAcceptance(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	cli := os.Args[0]
	settings := contracts.OrcaSettings{AccessMode: "full_access", Transport: contracts.TransportSettings{Executable: cli}}
	f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
	f.request.Workflow = contracts.Workflow{ID: workflow.SingleExecution, Version: 1, PromptOverrides: map[string]string{"execute": "finish work"}}
	f.request.Publication = contracts.Publication{}
	f.input.Input = canonical(f.request)
	source := f.freeze()
	source.ManagedWorkspace = &runstate.ManagedWorkspace{Identity: "original-native-incarnation", OriginRunID: source.ID}
	copyPath := filepath.Join(t.TempDir(), "alternate-orca.exe")
	program, err := os.ReadFile(cli)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, program, 0700); err != nil {
		t.Fatal(err)
	}
	before := string(canonical(source))
	for _, transport := range []contracts.TransportSettings{
		{Executable: cli, Args: []string{"different-gateway"}},
		{Executable: cli, Args: []string{"other-routing-argument"}},
		{Executable: copyPath},
	} {
		t.Run(filepath.Base(transport.Executable)+"/"+strings.Join(transport.Args, ","), func(t *testing.T) {
			settings.Transport = transport
			f.request.Execution.Settings = canonical(settings)
			f.request.ReuseWorktreeFromRunID = source.ID
			f.input.Input = canonical(f.request)
			_, err := freezeWithWorkspace(context.Background(), f.state, f.input, source)
			fault, ok := err.(*contracts.Fault)
			if !ok || fault.Detail.Code != "workspace_identity_conflict" || !strings.Contains(fault.Detail.Message, "transport identity") || string(canonical(source)) != before {
				t.Fatal("changed transport was not rejected before adoption or changed its source", err)
			}
		})
	}
}

func TestWorktreeReusePreservesCumulativeStateAndFreshIdentities(t *testing.T) {
	f, source := retainedFixture(t)
	root := source.Snapshot.Workspace.Path
	f.git(root, "config", "user.name", "Fixture")
	f.git(root, "config", "user.email", "fixture@example.invalid")
	for round := 0; round < 3; round++ {
		if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte("cumulative"), 0600); err != nil {
			t.Fatal(err)
		}
		f.git(root, "add", "spec.md")
		f.git(root, "commit", "--allow-empty", "-m", "round")
		if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte("unfinished"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "untracked"), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
		before := source.Clone()
		head := f.git(root, "rev-parse", "HEAD")
		status := f.git(root, "status", "--porcelain")
		f.request.ReuseWorktreeFromRunID = source.ID
		f.request.Workflow.PromptOverrides["execute"] = "new intent"
		f.input.Input = canonical(f.request)
		next, err := freezeWithWorkspace(context.Background(), f.state, f.input, source)
		if err != nil {
			t.Fatal(err)
		}
		if next.ID == source.ID || next.Snapshot.Session.AttemptID == source.Snapshot.Session.AttemptID || next.Snapshot.ResourceGeneration == source.Snapshot.ResourceGeneration || next.InputDigest == source.InputDigest || next.Snapshot.Session.SessionID != "" || len(next.Intents) != 0 || len(next.Workflow.StageInputs) != 0 {
			t.Fatal("old execution identity inherited")
		}
		if next.Snapshot.Workspace != source.Snapshot.Workspace || next.ManagedWorkspace.Identity != source.ManagedWorkspace.Identity || next.ManagedWorkspace.SourceRunID != source.ID || !reflect.DeepEqual(before, source) {
			t.Fatal("workspace or source changed")
		}
		if f.git(root, "rev-parse", "HEAD") != head || f.git(root, "status", "--porcelain") != status {
			t.Fatal("retained Git state changed")
		}
		if err := (&Engine{ctx: context.Background(), dir: f.state}).attachWriteRoots(next, false); err != nil {
			t.Fatal("new writer lacks retained Git roots", err)
		}
		next.Allowed, next.Closed, next.StopStatus, next.Status = false, true, "stopped", "stopped"
		source = next
	}
}

func TestWorktreeReuseRefusesUnknownTransferredAndMissingIncarnations(t *testing.T) {
	f, source := retainedFixture(t)
	e, _, _ := operationEngine(t)
	e.runs[source.ID] = source
	e.workers[source.ID] = &worker{wake: make(chan struct{}, 1)}
	for _, state := range []string{"unknown", "pending", ""} {
		source.StopStatus = state
		if _, unlock, err := e.reusableSource(source.ID); err == nil {
			unlock()
			t.Fatal("unsettled writer accepted", state)
		}
	}
	source.StopStatus = "stopped"
	source.Intents = []operation.Intent{{Request: b.LifecycleRequest{Operation: b.Operation{Status: b.OperationUnknown}}}}
	if _, unlock, err := e.reusableSource(source.ID); err == nil {
		unlock()
		t.Fatal("unknown original execution accepted")
	}
	source.Intents = nil
	source.CommandStates["original-stop"] = "unknown"
	if _, unlock, err := e.reusableSource(source.ID); err == nil {
		unlock()
		t.Fatal("unknown original command accepted")
	}
	delete(source.CommandStates, "original-stop")
	next := source.Clone()
	next.ID = "successor"
	next.ManagedWorkspace.SourceRunID = source.ID
	e.runs[next.ID] = next
	if _, unlock, err := e.reusableSource(source.ID); err == nil {
		unlock()
		t.Fatal("ancestor accepted")
	}
	if err := e.commitLocked(source.Clone(), nil, "late_cleanup"); err == nil {
		t.Fatal("late original mutation committed")
	}
	delete(e.runs, next.ID)
	unrelated := source.Clone()
	unrelated.ID = "competing-writer"
	unrelated.ManagedWorkspace.SourceRunID = ""
	unrelated.ManagedWorkspace.OriginRunID = unrelated.ID
	e.runs[unrelated.ID] = unrelated
	if _, unlock, err := e.reusableSource(source.ID); err == nil {
		unlock()
		t.Fatal("same incarnation acquired through another Run identity")
	}
	delete(e.runs, unrelated.ID)
	f.request.ReuseWorktreeFromRunID = source.ID
	f.input.Input = canonical(f.request)
	if err := os.Rename(source.Snapshot.Workspace.Path, source.Snapshot.Workspace.Path+"-retained"); err != nil {
		t.Fatal(err)
	}
	if _, err := freezeWithWorkspace(context.Background(), f.state, f.input, source); err == nil {
		t.Fatal("removed workspace recreated")
	}
	root := source.Snapshot.Workspace.Path
	gitfile, err := os.ReadFile(filepath.Join(root+"-retained", ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".git"), gitfile, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = freezeWithWorkspace(context.Background(), f.state, f.input, source); err == nil {
		t.Fatal("replacement checkout at the same Git path accepted")
	}
}

func TestWorktreeReusePreservesRebaseState(t *testing.T) {
	f, source := retainedFixture(t)
	admin := source.ManagedWorkspace.Local.AdminDirectory
	root := source.Snapshot.Workspace.Path
	f.git(root, "config", "user.name", "Rebase Fixture")
	f.git(root, "config", "user.email", "fixture@example.invalid")
	for _, side := range []struct{ path, body string }{{root, "retained side\n"}, {filepath.Join(f.root, "upstream"), "upstream side\n"}} {
		if side.path != root {
			f.git(source.RepositoryPath, "worktree", "add", "-b", "fixture-upstream", side.path, f.pinned)
		}
		if err := os.WriteFile(filepath.Join(side.path, "spec.md"), []byte(side.body), 0600); err != nil {
			t.Fatal(err)
		}
		f.git(side.path, "add", "spec.md")
		f.git(side.path, "commit", "-m", side.body)
	}
	if _, err := gitHost().Output(context.Background(), root, "rebase", "fixture-upstream"); err == nil {
		t.Fatal("fixture did not produce a real rebase conflict")
	}
	conflict, err := os.ReadFile(filepath.Join(root, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	head, status := f.git(root, "rev-parse", "HEAD"), f.git(root, "status", "--porcelain")
	f.request.ReuseWorktreeFromRunID = source.ID
	f.input.Input = canonical(f.request)
	next, err := freezeWithWorkspace(context.Background(), f.state, f.input, source)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Engine{ctx: context.Background(), dir: f.state}).verifyRecoveredStartup(next); err != nil {
		t.Fatal("rebase blocked", err)
	}
	if err := (&Engine{ctx: context.Background(), dir: f.state}).attachWriteRoots(next, false); err != nil {
		t.Fatal("rebase admin roots unavailable", err)
	}
	if err := verifyOriginalWorkspace(context.Background(), gitHost(), next, true, false); err != nil {
		t.Fatal("rebase result channel lost provenance", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "spec.md"))
	if err != nil || string(after) != string(conflict) || f.git(root, "rev-parse", "HEAD") != head || f.git(root, "status", "--porcelain") != status {
		t.Fatal("rebase conflict state changed", err)
	}
	if _, err := os.Stat(filepath.Join(admin, "rebase-merge", "head-name")); err != nil {
		t.Fatal("rebase state lost", err)
	}
}
