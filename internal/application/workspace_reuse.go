package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func workspaceSetup(r *runstate.Record) b.LifecycleSetup {
	return b.LifecycleSetup{Snapshot: r.Snapshot.Clone(), ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest, ProcessHost: process.New(30 * time.Second), RepositoryPath: r.Request.Repository.LocalPath, RepositoryURL: r.Request.Repository.URL}
}

// Called under mu. A successor remains the owner even after it stops: callers
// must reference that Run to continue the chain, never fork an ancestor.
func (e *Engine) workspaceSuperseded(r *runstate.Record) bool {
	if r.ManagedWorkspace == nil {
		return false
	}
	identity := r.ManagedWorkspace.Identity
	ancestors := map[string]bool{r.ID: true}
	for parent := r.ManagedWorkspace.SourceRunID; parent != ""; {
		prior := e.runs[parent]
		if ancestors[parent] || prior == nil || prior.ManagedWorkspace == nil || prior.ManagedWorkspace.Identity != identity {
			return true
		}
		ancestors[parent] = true
		parent = prior.ManagedWorkspace.SourceRunID
	}
	for _, other := range e.runs {
		// Arbitration uses the stable incarnation even if a competing record
		// did not reference the same source Run. Only proven ancestors are exempt.
		if other.ManagedWorkspace != nil && other.ManagedWorkspace.Identity == identity && !ancestors[other.ID] {
			return true
		}
	}
	return false
}

func (e *Engine) reusableSource(id string) (*runstate.Record, func(), error) {
	e.mu.Lock()
	w := e.workers[id]
	e.mu.Unlock()
	if w == nil {
		return nil, nil, contracts.Fail(5, "run_not_found", "Source Run was not found in this state directory.")
	}
	w.gate.Lock()
	unlock := func() { w.gate.Unlock() }
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id]
	if e.workspaceSuperseded(r) {
		unlock()
		return nil, nil, contracts.Fail(5, "workspace_already_reused", "Reference the latest Run holding this workspace.")
	}
	if r.WorkspaceWriteLeaseID != "" || r.StopStatus != "stopped" || r.Allowed || (r.Status != "stopped" && r.Status != "completed" && !r.Closed) || r.PendingStop != "" || r.PendingResume != "" || r.PendingInterrupt != "" || unknown(r) || w.analysisTasks != 0 || w.filePreparationsActive != 0 {
		e.signal(w)
		unlock()
		return nil, nil, contracts.Fail(5, "workspace_not_settled", "Stop and reconcile the original Run before reusing its workspace.")
	}
	for _, p := range []*runstate.Progress{r.Progress, fileState(r, true)} {
		if p != nil && p.InitializationAttempted && p.InitializationWriteStatus != "not_started" && p.InitializationWriteStatus != "settled" {
			unlock()
			return nil, nil, contracts.Fail(5, "workspace_not_settled", "Original optional file write is unverified.")
		}
	}
	for _, status := range r.CommandStates {
		if status == "pending" || status == "running" || status == "unknown" {
			unlock()
			return nil, nil, contracts.Fail(5, "workspace_not_settled", "Original command obligations remain unresolved.")
		}
	}
	if w.acquisition != nil {
		select {
		case <-w.acquisition.done:
		default:
			unlock()
			return nil, nil, contracts.Fail(5, "workspace_not_settled", "Original internal model call has not exited.")
		}
	}
	if r.ManagedWorkspace == nil || r.ManagedWorkspace.Identity == "" || r.Snapshot.Workspace.Path == "" {
		unlock()
		return nil, nil, contracts.Fail(5, "workspace_identity_unavailable", "Source Run has no retained managed workspace incarnation.")
	}
	if r.Request.ResultFile != nil && (r.ResultFile == nil || !r.ResultFile.Sealed) {
		unlock()
		return nil, nil, contracts.Fail(5, "workspace_not_settled", "Original result-file seal must finish before transfer.")
	}
	return r.Clone(), unlock, nil
}

func inheritWorkspace(ctx context.Context, r, source *runstate.Record) error {
	fail := func(err error) error {
		return contracts.Fail(5, "workspace_identity_conflict", "Managed worktree is missing, changed or unverified: "+err.Error())
	}
	if source.ManagedWorkspace == nil {
		return fail(errors.New("incarnation unavailable"))
	}
	if source.Request.Execution.Adapter == "orca-terminal" && r.Snapshot.Ref.Target.ID != source.Snapshot.Ref.Target.ID {
		return fail(errors.New("Orca transport identity changed; retain the original executable and arguments"))
	}
	r.RepositoryPath, r.RepositoryCommon = source.RepositoryPath, source.RepositoryCommon
	r.ManagedWorkspace = source.Clone().ManagedWorkspace
	r.ManagedWorkspace.SourceRunID = source.ID
	r.Snapshot.Workspace = source.Snapshot.Workspace
	r.Snapshot.WorkspacePlan = source.Snapshot.Workspace
	r.Snapshot.Ref.WorkspacePath = source.Snapshot.Workspace.Path
	r.Snapshot.Ref.WorkspaceBranch = source.Snapshot.Workspace.Branch
	r.WorkspaceFact = source.Clone().WorkspaceFact
	if source.Request.Execution.Adapter == "orca-terminal" {
		identity, err := orca.ManagedWorkspaceIdentity(workspaceSetup(source))
		if err != nil {
			return fail(err)
		}
		if identity != source.ManagedWorkspace.Identity {
			return fail(errors.New("native incarnation changed"))
		}
		if err := orca.ReuseManagedWorkspace(ctx, workspaceSetup(source), &r.Snapshot); err != nil {
			return fail(err)
		}
	} else {
		// A stopped label alone is insufficient once a native owner existed.
		if (source.Snapshot.Ref.External.Kind != "" || codex.ExecutionOwner(source.Snapshot) != nil) && !codexOwnerStopped(source) && !managedRecordWritersAbsent(source, source.Snapshot.Ref.External, func(p *b.ProcessIdentity) bool { return process.NewServiceFacts().Probe(p).State == "absent" }) {
			return contracts.Fail(5, "workspace_not_settled", "Original native writer absence is unverified.")
		}
		if err := verifyRetainedWorkspace(ctx, source); err != nil {
			return fail(err)
		}
	}
	return nil
}

func verifyRetainedWorkspace(ctx context.Context, r *runstate.Record) error {
	if r.ManagedWorkspace == nil {
		return errors.New("workspace incarnation unavailable")
	}
	remote, err := gitHost().Output(ctx, r.RepositoryPath, "config", "--get", "remote."+r.Request.Repository.Remote+".url")
	if err != nil || strings.TrimSpace(string(remote)) != r.Request.Repository.URL {
		return errors.New("managed repository remote changed")
	}
	return workspace.VerifyRetained(ctx, gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, &r.Snapshot.Workspace, r.ManagedWorkspace.Local)
}

func captureManagedWorkspace(ctx context.Context, r *runstate.Record) {
	if r.ManagedWorkspace != nil || r.Snapshot.Workspace.Path == "" {
		return
	}
	m := &runstate.ManagedWorkspace{OriginRunID: r.ID}
	if r.Request.Execution.Adapter == "orca-terminal" {
		m.Identity, _ = orca.ManagedWorkspaceIdentity(workspaceSetup(r))
	} else {
		m.Local, _ = workspace.CaptureRetainedIdentity(ctx, gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, &r.Snapshot.Workspace)
		if m.Local != nil {
			m.Identity = "local:" + strings.ToLower(r.RepositoryCommon) + ":" + m.Local.Directory + ":" + m.Local.Admin
		}
	}
	if m.Identity != "" {
		r.ManagedWorkspace = m
	}
}
