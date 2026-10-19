package application

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
