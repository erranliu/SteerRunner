package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Orca local creates a worktree of the caller's frozen registered checkout,
// whereas Record.RepositoryCommon is the material cache's logical project ID.
// Resolve that original checkout and require the actual worktree to share its
// Git common directory. Never resolve an Orca environment path on this host.
func evidenceRepository(ctx context.Context, git workspace.Git, r *runstate.Record) (workspace.Repository, error) {
	if r.Request.Execution.Adapter != "orca-terminal" {
		return workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, nil
	}
	if r.Request.Execution.Target.Kind != "local" {
		return workspace.Repository{}, contracts.Fail(3, "unsupported_capability", "Remote workspace evidence is unsupported.")
	}
	repo, err := workspace.ResolveRepository(ctx, git, r.Request.Repository.LocalPath)
	if err != nil || !workspace.SamePath(repo.Path, r.Request.Repository.LocalPath) {
		return workspace.Repository{}, contracts.Fail(4, "workspace_unavailable", "Original registered local checkout is unavailable or redirected.")
	}
	remote, err := git.Output(ctx, repo.Path, "config", "--get", "remote."+r.Request.Repository.Remote+".url")
	if err != nil || strings.TrimSpace(string(remote)) != r.Request.Repository.URL {
		return workspace.Repository{}, contracts.Fail(5, "workspace_source_unknown", "Original local Orca repository source differs from the frozen Run.")
	}
	return repo, nil
}

func verifyEvidenceWorkspace(ctx context.Context, git workspace.Git, r *runstate.Record) error {
	return verifyOriginalWorkspace(ctx, git, r, false)
}
func verifyOriginalWorkspace(ctx context.Context, git workspace.Git, r *runstate.Record, fileChannel bool) error {
	if workflow.Content(r.Request.Workflow.ID) || r.Request.Execution.Target.Kind != "local" || (r.Request.Execution.Adapter != "codex-app-server" && r.Request.Execution.Adapter != "orca-terminal") {
		return contracts.Fail(3, "unsupported_capability", "R2 workspace evidence supports host-local development workspaces only; remote paths are never read locally.")
	}
	s := r.Snapshot
	f := r.WorkspaceFact
	repositoryMatches := f != nil && (workspace.SamePath(f.Repository, r.RepositoryPath) || r.Request.Execution.Adapter == "orca-terminal" && (workspace.SamePath(f.Repository, r.RepositoryCommon) || workspace.SamePath(f.Repository, s.Ref.WorkspacePath)))
	if s.Ref.Incomplete || s.Ref.RunID != r.ID || s.Session.WorkflowRunID != r.ID || s.Session.ExecutionID != r.ID || !workspace.SamePath(s.Session.ProjectID, r.RepositoryCommon) || !workspace.SamePath(s.Ref.WorkspacePath, s.Workspace.Path) || s.Ref.WorkspaceBranch != s.Workspace.Branch || s.Ref.BaseCommit != s.Workspace.BaseCommit || s.Ref.BaseCommit != s.WorkspacePlan.BaseCommit || f == nil || !workspace.SamePath(f.Worktree, s.Ref.WorkspacePath) || f.Branch != s.Ref.WorkspaceBranch || f.Base != s.Ref.BaseCommit || !repositoryMatches {
		return contracts.Fail(5, "workspace_source_unknown", "Original Run workspace provenance is missing or inconsistent.")
	}
	repo, err := evidenceRepository(ctx, git, r)
	if err != nil {
		return err
	}
	verify := func() error { return workspace.Verify(ctx, git, repo, &s.Workspace, false) }
	if fileChannel {
		verify = func() error { return workspace.VerifyFileSource(ctx, git, repo, &s.Workspace) }
	}
	if err := verify(); err != nil {
		return contracts.Fail(4, "workspace_unavailable", "Original Run workspace cannot be verified: "+redactDiagnostic(r, err.Error()))
	}
	return nil
}

// Evidence collection has no worker wake, native port, resume or business input.
// The existing gate/Commit/request lock provide stop ordering, CAS and replay.
func (e *Engine) HandleEvidence(command, runID, artifactID string, input *contracts.Request) contracts.Response {
	return e.handleEvidence(command, runID, artifactID, input, gitHost())
}

func (e *Engine) handleEvidence(command, runID, artifactID string, input *contracts.Request, git workspace.Git) contracts.Response {
	id := ""
	if input != nil {
		id = input.RequestID
	}
	reject := func(err error) contracts.Response { return contracts.Rejected(id, fault(err)) }
	if command == "artifacts collect" {
		if input == nil {
			return reject(contracts.Fail(2, "invalid_input", "Collection requires a write request."))
		}
		if invalid := contracts.ValidateInput(command, input.Input); invalid != nil {
			return reject(invalid)
		}
		entry, _ := e.requests.LoadOrStore(id, &sync.Mutex{})
		lock := entry.(*sync.Mutex)
		lock.Lock()
		defer lock.Unlock()
		digest := storage.RequestDigest(command, runID, *input)
		if original, found, err := e.store.Original(id, digest); err != nil {
			return reject(err)
		} else if found {
			return original
		}
		e.mu.Lock()
		w := e.workers[runID]
		e.mu.Unlock()
		if w == nil {
			return reject(contracts.Fail(5, "run_not_found", "Run was not found."))
		}
		w.gate.Lock()
		defer w.gate.Unlock()
	} else if input != nil {
		return reject(contracts.Fail(2, "invalid_input", "Evidence queries do not accept input."))
	}
	e.mu.Lock()
	var r *runstate.Record
	if current := e.runs[runID]; current != nil {
		r = current.Clone()
	}
	e.mu.Unlock()
	if r == nil {
		return reject(contracts.Fail(5, "run_not_found", "Run was not found."))
	}
	switch command {
	case "artifacts list":
		out := contracts.ArtifactList{RunID: runID, Artifacts: []contracts.Artifact{}}
		for _, a := range r.Artifacts {
			out.Artifacts = append(out.Artifacts, a.Artifact)
		}
		return contracts.Success("", out)
	case "artifacts read":
		for _, a := range r.Artifacts {
			if a.ArtifactID == artifactID && a.RunID == runID {
				if a.SizeBytes != len(a.Content) || a.SizeBytes > contracts.MaxArtifactBytes || reliable.Digest(a.Content) != a.SHA256 {
					return reject(contracts.Fail(5, "artifact_integrity_failed", "Collected artifact bytes do not match their immutable metadata."))
				}
				return contracts.Success("", contracts.ArtifactContent{Artifact: a.Artifact, Encoding: "base64", ContentBase64: base64.StdEncoding.EncodeToString(a.Content)})
			}
		}
		return reject(contracts.Fail(5, "artifact_not_found", "Artifact identity does not belong to this Run."))
	}
	if readOnlyRun(r) {
		return reject(contracts.Fail(3, "unsupported_capability", retiredAccessDiagnostic))
	}
	ctx, cancel := context.WithTimeout(e.ctx, 20*time.Second)
	defer cancel()
	if command == "workspace-git" {
		out := contracts.WorkspaceGit{RunID: runID}
		err := verifyEvidenceWorkspace(ctx, git, r)
		if err == nil {
			var commit string
			var clean bool
			var repo workspace.Repository
			repo, err = evidenceRepository(ctx, git, r)
			if err == nil {
				commit, clean, err = workspace.CurrentGit(ctx, git, repo, &r.Snapshot.Workspace)
			}
			if err == nil {
				out.Commit = &commit
				out.Clean = &clean
				out.Verified = true
			}
		}
		if err == nil {
			err = e.checkEvidenceIdentity(r)
		}
		if err != nil {
			out.Verified, out.Commit, out.Clean = false, nil, nil
		}
		out.ObservedAt = time.Now().UTC()
		if err != nil {
			out.Diagnostic = redactDiagnostic(r, err.Error())
		}
		return contracts.Success("", out)
	}
	if err := verifyEvidenceWorkspace(ctx, git, r); err != nil {
		return reject(err)
	}
	switch command {
	case "artifacts discover":
		candidates, err := workspace.DiscoverArtifacts(r.Snapshot.Ref.WorkspacePath)
		if err != nil {
			return reject(err)
		}
		if err := verifyEvidenceWorkspace(ctx, git, r); err != nil {
			return reject(err)
		}
		if err := e.checkEvidenceIdentity(r); err != nil {
			return reject(err)
		}
		return contracts.Success("", contracts.ArtifactDiscovery{RunID: runID, Directory: contracts.ArtifactDirectory, Candidates: candidates})
	case "artifacts collect":
		if e.ctx.Err() != nil {
			return reject(contracts.Fail(4, "service_unavailable", "Service is stopping; collection was not accepted."))
		}
		var value contracts.CollectArtifactInput
		_ = json.Unmarshal(input.Input, &value)
		if value.ExpectedRevision != r.Revision {
			return reject(contracts.Fail(5, "stale_revision", "Run revision changed; inspect before a new collection request."))
		}
		total := 0
		for _, a := range r.Artifacts {
			total += a.SizeBytes
		}
		if len(r.Artifacts) >= contracts.MaxRunArtifacts {
			return reject(contracts.Fail(2, "artifact_limit_exceeded", "Run has reached its 64 artifact limit."))
		}
		data, err := workspace.ReadArtifactSource(r.Snapshot.Ref.WorkspacePath, value.RelativePath)
		if err != nil {
			return reject(err)
		}
		if total+len(data) > contracts.MaxRunArtifactBytes {
			return reject(contracts.Fail(2, "artifact_limit_exceeded", "Run artifacts exceed 16 MiB."))
		}
		if err := verifyEvidenceWorkspace(ctx, git, r); err != nil {
			return reject(err)
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		current := e.runs[runID]
		if current == nil || current.Revision != r.Revision {
			return reject(contracts.Fail(5, "stale_revision", "Run revision changed; inspect before a new collection request."))
		}
		if err := evidenceIdentity(current, r); err != nil {
			return reject(err)
		}
		if e.ctx.Err() != nil {
			return reject(contracts.Fail(4, "service_unavailable", "Service is stopping; collection was not accepted."))
		}
		// Revision equality preserves the artifact count/byte limits sampled above.
		receipt := storage.NewCommand(command, runID, *input)
		a := contracts.Artifact{RunID: runID, ArtifactID: contracts.NewID(), RelativePath: value.RelativePath, SHA256: reliable.Digest(data), SizeBytes: len(data), CollectedAt: time.Now().UTC()}
		a.ReadCommand = fmt.Sprintf("artifacts read --run-id %s --artifact-id %s", runID, a.ArtifactID)
		next := r.Clone()
		next.Artifacts = append(next.Artifacts, runstate.Artifact{Artifact: a, Content: data})
		next.CommandStates[receipt.ID] = "completed"
		receipt.Status = "completed"
		receipt.Response = contracts.Success(id, map[string]any{"command_id": receipt.ID, "run_id": runID, "receipt_status": "accepted", "action_status": "completed", "artifact": a})
		if err := e.commitLocked(next, &receipt, "artifact_collected"); err != nil {
			return reject(contracts.Fail(6, "request_outcome_unknown", "Collection commit is unverified; inspect or replay the original request_id."))
		}
		return receipt.Response
	default:
		return reject(contracts.Fail(2, "invalid_arguments", "Unknown evidence command."))
	}
}

// Evidence is scoped to the original workspace, independently of input authority
// or a current native turn. Slow reads may overlap stop and native observations.
func evidenceIdentity(current, original *runstate.Record) error {
	if current == nil || current.ID != original.ID || current.RepositoryPath != original.RepositoryPath || current.RepositoryCommon != original.RepositoryCommon || (current.Request.Workflow.ID != original.Request.Workflow.ID || current.Request.Workflow.Version != original.Request.Workflow.Version) || current.Request.Repository != original.Request.Repository || (current.InputDigest != original.InputDigest || current.Request.Execution.Adapter != original.Request.Execution.Adapter || current.Request.Execution.Target != original.Request.Execution.Target) || current.Snapshot.Ref != original.Snapshot.Ref || current.Snapshot.Workspace != original.Snapshot.Workspace || current.Snapshot.WorkspacePlan != original.Snapshot.WorkspacePlan || current.Snapshot.Session.ProjectID != original.Snapshot.Session.ProjectID || current.Snapshot.Session.WorkflowRunID != original.Snapshot.Session.WorkflowRunID || current.Snapshot.Session.ExecutionID != original.Snapshot.Session.ExecutionID || !sameEvidenceWorkspace(current.WorkspaceFact, original.WorkspaceFact) {
		return contracts.Fail(5, "workspace_source_unknown", "Original Run workspace provenance changed during observation.")
	}
	return nil
}
func (e *Engine) checkEvidenceIdentity(original *runstate.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return evidenceIdentity(e.runs[original.ID], original)
}

func sameEvidenceWorkspace(a, z *b.WorkspaceInspection) bool {
	if a == nil || z == nil {
		return a == nil && z == nil
	}
	return a.Repository == z.Repository && a.Worktree == z.Worktree && a.Branch == z.Branch && a.Base == z.Base && a.InspectionScope == z.InspectionScope
}
