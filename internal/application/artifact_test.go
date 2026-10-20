package application

import (
	"database/sql"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestR2UnknownSourceAndReadOnlyQueries(t *testing.T) {
	e, w := auditEngine(t)
	r := e.runs["run"]
	s := &r.Snapshot
	s.Workspace = s.WorkspacePlan
	s.Ref = b.ExecutionRef{RunID: r.ID, WorkspacePath: s.WorkspacePlan.Path, WorkspaceBranch: s.WorkspacePlan.Branch, BaseCommit: s.WorkspacePlan.BaseCommit}
	s.Session.WorkflowRunID = r.ID
	s.Session.ExecutionID = r.ID
	s.Session.ProjectID = r.RepositoryCommon
	r.WorkspaceFact = &b.WorkspaceInspection{Repository: r.RepositoryPath, Worktree: s.Ref.WorkspacePath, Branch: s.Ref.WorkspaceBranch, Base: s.Ref.BaseCommit}
	// Any unexpected native call panics; queries have no native capability.
	w.port = nil
	before := r.Clone()
	response := e.HandleEvidence("workspace-git", r.ID, "", nil)
	out := response.Data.(contracts.WorkspaceGit)
	if !out.Verified || out.Clean == nil || !*out.Clean || out.Commit == nil {
		t.Fatal(response)
	}
	if !reflect.DeepEqual(before, r.Clone()) || len(w.wake) != 0 {
		t.Fatal("query mutated or woke Run")
	}
	s.Ref.RunID = "other"
	out = e.HandleEvidence("workspace-git", r.ID, "", nil).Data.(contracts.WorkspaceGit)
	if out.Verified || out.Clean != nil || out.Commit != nil || out.Diagnostic == "" {
		t.Fatal(out)
	}
	response = e.HandleEvidence("artifacts discover", r.ID, "", nil)
	if response.OK || response.Error.Code != "workspace_source_unknown" {
		t.Fatal(response)
	}
	s.Ref.RunID = r.ID
	r.Request.Execution.Target.Kind = "orca_environment"
	out = e.HandleEvidence("workspace-git", r.ID, "", nil).Data.(contracts.WorkspaceGit)
	if out.Verified || out.Clean != nil {
		t.Fatal(out)
	}
	response = e.HandleEvidence("artifacts discover", r.ID, "", nil)
	if response.OK || response.Error.Code != "unsupported_capability" {
		t.Fatal(response)
	}
	r.Request.Execution.Target.Kind = "local"
	// Orca local owns an actual worktree distinct from its original plan path;
	// its logical project/receipt repository is the material cache identity.
	r.Request.Execution.Adapter = "orca-terminal"
	r.Request.Repository.LocalPath = r.RepositoryPath
	r.Request.Repository.Remote = "origin"
	r.Request.Repository.URL = "https://example.invalid/fixture/repo.git"
	gitTest(t, r.RepositoryPath, "remote", "add", "origin", r.Request.Repository.URL)
	r.WorkspaceFact.Repository = r.RepositoryCommon
	s.WorkspacePlan.Path = filepath.Join(t.TempDir(), "planned-only")
	out = e.HandleEvidence("workspace-git", r.ID, "", nil).Data.(contracts.WorkspaceGit)
	if !out.Verified || out.Clean == nil || !*out.Clean {
		t.Fatal("original Orca local workspace was not observed", out)
	}
	r.Request.Execution.Adapter = "codex-app-server"
	r.WorkspaceFact.Repository = r.RepositoryPath
	root := filepath.Join(s.Ref.WorkspacePath, filepath.FromSlash(contracts.ArtifactDirectory))
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "report"), []byte("program report"), 0600); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(contracts.CollectArtifactInput{ExpectedRevision: r.Revision, RelativePath: "report"})
	request := &contracts.Request{Protocol: contracts.Protocol, RequestID: "collect", Input: input}
	response = e.HandleEvidence("artifacts collect", r.ID, "", request)
	if !response.OK {
		t.Fatal(response)
	}
	next := e.runs[r.ID]
	a := next.Artifacts[0]
	if next.PendingResume != "" || next.PendingFeedback != nil || len(w.wake) != 0 {
		t.Fatal("collection dispatched work")
	}
	records, err := e.store.Runs()
	if err != nil || len(records) != 1 || !reflect.DeepEqual(records[0].Artifacts, next.Artifacts) {
		t.Fatal("evidence not durable", err)
	}
	beforeBytes, _ := json.Marshal(e.HandleEvidence("artifacts read", r.ID, a.ArtifactID, nil))
	// Stop/cleanup facts never authorize overwriting immutable records.
	next.StopStatus = "stopped"
	next.Allowed = false
	next.ResumeRequired = true
	if err := os.Remove(filepath.Join(root, "report")); err != nil {
		t.Fatal(err)
	}
	replay := e.HandleEvidence("artifacts collect", r.ID, "", request)
	originalJSON, _ := json.Marshal(response)
	replayJSON, _ := json.Marshal(replay)
	var originalValue, replayValue any
	_ = json.Unmarshal(originalJSON, &originalValue)
	_ = json.Unmarshal(replayJSON, &replayValue)
	if !reflect.DeepEqual(originalValue, replayValue) {
		t.Fatal("replay requires deleted source", replay)
	}
	afterBytes, _ := json.Marshal(e.HandleEvidence("artifacts read", r.ID, a.ArtifactID, nil))
	if string(beforeBytes) != string(afterBytes) {
		t.Fatal("stop changed evidence")
	}
	next.Artifacts[0].Content[0] = 'X'
	response = e.HandleEvidence("artifacts read", r.ID, a.ArtifactID, nil)
	if response.OK || response.Error.Code != "artifact_integrity_failed" {
		t.Fatal(response)
	}
}

func evidenceEngine(t *testing.T) (*Engine, *worker) {
	t.Helper()
	e, w := auditEngine(t)
	r := e.runs["run"]
	s := &r.Snapshot
	s.Workspace = s.WorkspacePlan
	s.Ref = b.ExecutionRef{RunID: r.ID, WorkspacePath: s.Workspace.Path, WorkspaceBranch: s.Workspace.Branch, BaseCommit: s.Workspace.BaseCommit}
	s.Session.WorkflowRunID, s.Session.ExecutionID, s.Session.ProjectID = r.ID, r.ID, r.RepositoryCommon
	r.WorkspaceFact = &b.WorkspaceInspection{Repository: r.RepositoryPath, Worktree: s.Workspace.Path, Branch: s.Workspace.Branch, Base: s.Workspace.BaseCommit}
	root := filepath.Join(s.Workspace.Path, filepath.FromSlash(contracts.ArtifactDirectory))
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "report"), []byte("original bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	return e, w
}
func evidenceRequest(rid string, revision uint64, collect bool) *contracts.Request {
	var value any = contracts.RevisionInput{ExpectedRevision: revision}
	if collect {
		value = contracts.CollectArtifactInput{ExpectedRevision: revision, RelativePath: "report"}
	}
	raw, _ := json.Marshal(value)
	return &contracts.Request{Protocol: contracts.Protocol, RequestID: rid, Input: raw}
}
func blockedEvidenceGit(t *testing.T) (workspace.Git, <-chan struct{}, func()) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	git := gitHost()
	start := git.Start
	git.Start = func(cmd *exec.Cmd) (b.StopHandle, error) {
		enteredOnce.Do(func() { close(entered); <-release })
		return start(cmd)
	}
	return git, entered, unblock
}
func evidenceResponse(t *testing.T, done <-chan contracts.Response) contracts.Response {
	t.Helper()
	select {
	case response := <-done:
		return response
	case <-time.After(30 * time.Second):
		t.Fatal("evidence/control remained blocked")
		return contracts.Response{}
	}
}
func awaitEvidenceIO(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("evidence query did not reach I/O barrier")
	}
}
func TestR2SlowEvidenceDoesNotBlockOtherRunControl(t *testing.T) {
	for _, command := range []string{"workspace-git", "artifacts discover", "artifacts collect"} {
		t.Run(command, func(t *testing.T) {
			e, w := evidenceEngine(t)
			other := e.runs["run"].Clone()
			other.ID = "other"
			e.mu.Lock()
			e.workers["other"] = &worker{wake: make(chan struct{}, 1)}
			if err := e.commitLocked(other, nil, "accepted"); err != nil {
				t.Fatal(err)
			}
			e.mu.Unlock()
			git, entered, release := blockedEvidenceGit(t)
			done := make(chan contracts.Response, 1)
			var request *contracts.Request
			if command == "artifacts collect" {
				request = evidenceRequest("collect", e.runs["run"].Revision, true)
			}
			go func() { done <- e.handleEvidence(command, "run", "", request, git) }()
			awaitEvidenceIO(t, entered)
			if command == "artifacts collect" && w.gate.TryLock() {
				w.gate.Unlock()
				t.Fatal("collection lost its same-Run stop gate")
			}
			stopped := make(chan contracts.Response, 1)
			go func() {
				stopped <- e.Handle("stop", "other", "", "", "", 0, false, evidenceRequest("stop-other", other.Revision, false))
			}()
			if response := evidenceResponse(t, stopped); !response.OK {
				t.Fatal(response)
			}
			committed := make(chan error, 1)
			go func() {
				e.mu.Lock()
				defer e.mu.Unlock()
				next := e.runs["other"].Clone()
				next.Diagnostic = "late original fact"
				committed <- e.commitLocked(next, nil, "observed")
			}()
			select {
			case err := <-committed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("other Run fact blocked by evidence I/O")
			}
			e.mu.Lock()
			revoked := !e.runs["other"].Allowed && e.runs["other"].PendingStop != ""
			e.mu.Unlock()
			if !revoked {
				t.Fatal("other Run stop was not durable")
			}
			release()
			if response := evidenceResponse(t, done); !response.OK {
				t.Fatal(response)
			}
		})
	}
}
func TestR2CollectionSerializesStopAndRejectsConcurrentRevision(t *testing.T) {
	for _, variant := range []string{"stop", "revision", "identity"} {
		t.Run(variant, func(t *testing.T) {
			e, _ := evidenceEngine(t)
			original := e.runs["run"].Clone()
			git, entered, release := blockedEvidenceGit(t)
			done := make(chan contracts.Response, 1)
			request := evidenceRequest("collect", original.Revision, true)
			go func() { done <- e.handleEvidence("artifacts collect", "run", "", request, git) }()
			awaitEvidenceIO(t, entered)
			var stopped chan contracts.Response
			if variant == "stop" {
				stopped = make(chan contracts.Response, 1)
				go func() {
					stopped <- e.Handle("stop", "run", "", "", "", 0, false, evidenceRequest("stop", original.Revision+1, false))
				}()
			} else {
				e.mu.Lock()
				next := e.runs["run"].Clone()
				if variant == "identity" {
					next.Snapshot.Ref.WorkspacePath = "different"
				}
				if err := e.commitLocked(next, nil, "observed"); err != nil {
					t.Fatal(err)
				}
				e.mu.Unlock()
			}
			release()
			response := evidenceResponse(t, done)
			if variant == "stop" {
				if !response.OK {
					t.Fatal(response)
				}
				if response = evidenceResponse(t, stopped); !response.OK {
					t.Fatal("stop did not follow collection", response)
				}
				if len(e.runs["run"].Artifacts) != 1 || e.runs["run"].Allowed {
					t.Fatal("collection/stop ordering changed")
				}
			} else if response.OK || response.Error.Code != "stale_revision" || len(e.runs["run"].Artifacts) != 0 {
				t.Fatal("stale source was committed", response)
			}
		})
	}
}
func TestR2QueryRejectsChangedWorkspaceButAllowsStop(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(map[bool]string{false: "stop", true: "identity"}[change], func(t *testing.T) {
			e, _ := evidenceEngine(t)
			git, entered, release := blockedEvidenceGit(t)
			done := make(chan contracts.Response, 1)
			go func() { done <- e.handleEvidence("workspace-git", "run", "", nil, git) }()
			awaitEvidenceIO(t, entered)
			e.mu.Lock()
			next := e.runs["run"].Clone()
			next.Allowed = false
			next.PendingStop = "stop"
			if change {
				next.Snapshot.Ref.WorkspacePath = "different"
			}
			if err := e.commitLocked(next, nil, "stop_requested"); err != nil {
				t.Fatal(err)
			}
			e.mu.Unlock()
			release()
			response := evidenceResponse(t, done)
			out := response.Data.(contracts.WorkspaceGit)
			if out.Verified == change || change && (out.Commit != nil || out.Clean != nil) {
				t.Fatal(out)
			}
		})
	}
}
func TestR2CollectionCommitFailureReplaysOriginalRequest(t *testing.T) {
	e, _ := evidenceEngine(t)
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	failOperationCommit(t, db)
	request := evidenceRequest("collect", e.runs["run"].Revision, true)
	response := e.HandleEvidence("artifacts collect", "run", "", request)
	if response.OK || response.Error.Code != "request_outcome_unknown" || len(e.runs["run"].Artifacts) != 0 {
		t.Fatal(response)
	}
	if _, err = db.Exec("DROP TRIGGER fail_operation_event"); err != nil {
		t.Fatal(err)
	}
	response = e.HandleEvidence("artifacts collect", "run", "", request)
	if !response.OK {
		t.Fatal(response)
	}
	var original, replayed any
	_ = json.Unmarshal(canonical(response), &original)
	replay := e.HandleEvidence("artifacts collect", "run", "", request)
	_ = json.Unmarshal(canonical(replay), &replayed)
	if !reflect.DeepEqual(original, replayed) || len(e.runs["run"].Artifacts) != 1 {
		t.Fatal(replay)
	}
}
