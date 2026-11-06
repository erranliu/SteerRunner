package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Stored inside the existing operation receipt, including on terminal-query failure.
// No resource is rediscovered by name when its creation response was lost.
type workspaceCreation struct {
	RuntimeID            string          `json:"runtime_id"`
	ResolvedRepositoryID string          `json:"resolved_repository_id,omitempty"`
	ResolvedRuntimeID    string          `json:"resolved_runtime_id,omitempty"`
	Worktree             native.Worktree `json:"worktree"`
	AgentTerminalHandle  string          `json:"agentTerminalHandle,omitempty"`
	StartupTerminal      native.Terminal `json:"startupTerminal"`
	Terminal             native.Terminal `json:"selected_terminal"`
}

func (l *Lifecycle) prepareWorkspace(ctx context.Context, r b.LifecycleRequest, reconcile bool) (receipt b.LifecycleReceipt, err error) {
	receipt = b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	receipt.Operation.Receipt = nil // This field contains the checkpoint, not another outer receipt.
	s := r.Snapshot
	if r.Input.Workspace == nil || *r.Input.Workspace != s.WorkspacePlan || !b.StableExecutionID(s.Session.WorkflowRunID) {
		return receipt, errors.New("workspace differs from original plan")
	}
	name := workspaceName(s)
	var created workspaceCreation
	// A durable preparation query proves this operation reached only discovery.
	// Any claimed creation directory wins: recover that exact plan, never select
	// another repository or relaunch an owner with an unknown result.
	key := l.creationKey(s, r.Operation.ID, r.Operation.PayloadDigest, "workspace")
	if _, statErr := os.Stat(creationDirectory(l.setup.RunDirectory, key)); statErr == nil {
		reconcile = true
	} else if reconcile && os.IsNotExist(statErr) && l.setup.LaunchExecutionOwner != nil && b.PreparationQueryRecorded(ctx) {
		if !b.PreparationRecovery(ctx) {
			// This owner creates its durable directory before any launch. The
			// original query plus absent directory proves creation never began;
			// revoked authorization permits settlement, never a query or create.
			receipt.Disposition = b.OperationRejected
			return receipt, nil
		}
		reconcile = false
	}
	if reconcile {
		var saved b.LifecycleReceipt
		if len(r.Operation.Receipt) != 0 && (json.Unmarshal(r.Operation.Receipt, &saved) != nil || saved.ValidateFor(r) != nil) {
			return receipt, errors.New("original workspace operation receipt invalid or foreign")
		}
		if len(saved.Operation.Receipt) == 0 {
			created, err = l.workspaceCreation(ctx, r, true)
			if err != nil {
				return receipt, err
			}
		} else {
			if json.Unmarshal(saved.Operation.Receipt, &created) != nil {
				return receipt, errors.New("original workspace creation checkpoint invalid")
			}
			// Keep the checkpoint even when this reconciliation cannot query Orca.
			receipt.Operation.Receipt = append([]byte(nil), saved.Operation.Receipt...)
		}
	} else {
		selector, runtimeID, resolveErr := l.workspaceRepository(ctx)
		if resolveErr != nil {
			// Only read-only discovery ran; no creation needs reconciliation.
			receipt.Disposition = b.OperationRejected
			return receipt, resolveErr
		}
		args := l.workspaceCreationArgs(r, selector)

		key := l.creationKey(r.Snapshot, r.Operation.ID, r.Operation.PayloadDigest, "workspace")
		result, callErr := l.creationCall(ctx, key, args, runtimeID, selector, l.creationTimeout(), false)
		if callErr != nil {
			return receipt, callErr
		}
		if created, err = bindWorkspaceCreation(result, selector, runtimeID); err != nil {
			return receipt, err
		}
	}
	worktree := created.Worktree
	if (l.profile.Environment != "" || created.ResolvedRuntimeID != "") && (created.ResolvedRuntimeID == "" || created.ResolvedRuntimeID != created.RuntimeID || l.profile.Environment != "" && created.ResolvedRepositoryID == "" || created.ResolvedRepositoryID != "" && created.ResolvedRepositoryID != worktree.RepoID) {
		// Keep both the discovery identity and creation response for reconciliation.
		receipt.Operation.Receipt, _ = json.Marshal(created)
		return receipt, errors.New("Orca workspace creation changed the resolved repository/runtime")
	}
	if created.RuntimeID == "" || worktree.ID == "" || worktree.InstanceID == "" || worktree.Path == "" || worktree.Head != r.Input.Workspace.BaseCommit || worktree.BaseRef != r.Input.Workspace.BaseCommit || worktree.DisplayName != name {
		return receipt, errors.New("Orca worktree differs from fixed name/base")
	}
	defer func() { receipt.Operation.Receipt, _ = json.Marshal(created) }()

	return l.finishStructuredWorkspace(ctx, r, created)

}

func workspaceCreationFacts(project string, target b.ExecutionTarget, worktree native.Worktree, base string, checkedAt time.Time) (b.WorkspaceInspection, b.ExecutionTarget) {
	workspace := b.WorkspaceInspection{InspectionScope: "revision", Repository: project, Worktree: worktree.Path, Branch: strings.TrimPrefix(worktree.Branch, "refs/heads/"), Base: base, Head: worktree.Head, CheckedAt: checkedAt}
	target.WorktreeID = worktree.ID
	target.Branch = workspace.Branch
	target.BaseRevision = workspace.Base
	return workspace, target
}

func (l *Lifecycle) creationTimeout() time.Duration {
	if l.setup.CommandTimeout > 0 {
		return l.setup.CommandTimeout
	}
	return 40 * time.Second
}

func (l *Lifecycle) workspaceCreationArgs(r b.LifecycleRequest, selector string) []string {
	return native.WorktreeCreateArgs(native.WorktreeCreate{Repository: selector, Name: workspaceName(r.Snapshot), BaseBranch: r.Input.Workspace.BaseCommit, NoParent: true, Setup: "skip"})
}

// Native creation fields and execution checkpoint extensions have different
// owners. Keep the latter's original decoding and persisted representation.
func decodeWorkspaceCreation(result native.Result) (workspaceCreation, error) {
	var created workspaceCreation
	value, err := native.DecodeWorktreeCreation(result)
	if err != nil {
		return created, err
	}
	var checkpoint struct {
		RuntimeID            string          `json:"runtime_id"`
		ResolvedRepositoryID string          `json:"resolved_repository_id,omitempty"`
		ResolvedRuntimeID    string          `json:"resolved_runtime_id,omitempty"`
		Terminal             native.Terminal `json:"selected_terminal"`
	}
	if err = json.Unmarshal(result.Payload, &checkpoint); err != nil {
		return created, err
	}
	created.Worktree, created.AgentTerminalHandle, created.StartupTerminal = value.Worktree, value.AgentTerminalHandle, value.StartupTerminal
	created.RuntimeID, created.ResolvedRepositoryID, created.ResolvedRuntimeID, created.Terminal = checkpoint.RuntimeID, checkpoint.ResolvedRepositoryID, checkpoint.ResolvedRuntimeID, checkpoint.Terminal
	return created, nil
}

func bindWorkspaceCreation(result native.Result, selector, runtimeID string) (workspaceCreation, error) {
	created, err := decodeWorkspaceCreation(result)
	if err != nil {
		return created, err
	}
	created.RuntimeID = result.RuntimeID
	if runtimeID != "" {
		created.ResolvedRuntimeID = runtimeID
		if strings.HasPrefix(selector, "id:") {
			created.ResolvedRepositoryID = strings.TrimPrefix(selector, "id:")
		}
	}
	return created, nil
}

func workspaceName(s b.LifecycleSnapshot) string {
	return s.Session.NamePrefix + "-" + s.Session.WorkflowRunID
}

// Repository resolution is an Execute-only preparation query. Recovery uses
// the original resolved selector/runtime from the Runner's immutable plan.
func (l *Lifecycle) workspaceCreation(ctx context.Context, r b.LifecycleRequest, reconcile bool) (workspaceCreation, error) {
	var created workspaceCreation
	key := l.creationKey(r.Snapshot, r.Operation.ID, r.Operation.PayloadDigest, "workspace")
	plan, err := l.originalCreationPlan(key)
	if err != nil {
		return created, errors.Join(err, errors.New("original workspace creation has no exact receipt; no worktree/agent was recreated"))
	}
	if plan.Runtime == "" || l.profile.Environment == "" && plan.Selector != "path:"+l.setup.RepositoryPath || l.profile.Environment != "" && !strings.HasPrefix(plan.Selector, "id:") {
		return created, errors.New("original resolved repository binding changed")
	}
	result, err := l.creationCall(ctx, key, l.workspaceCreationArgs(r, plan.Selector), plan.Runtime, plan.Selector, plan.Command.Timeout, reconcile)
	if err != nil {
		return created, err
	}
	return bindWorkspaceCreation(result, plan.Selector, plan.Runtime)
}

// Repository selection belongs to the project, not a reusable execution profile.
// An environment inventory is matched by directory basename, never displayName.
func (l *Lifecycle) workspaceRepository(ctx context.Context) (selector, runtimeID string, err error) {
	repository := l.setup.RepositoryPath
	if strings.TrimSpace(repository) == "" || strings.ContainsAny(repository, "\x00\r\n") {
		return "", "", errors.New("missing project repository path")
	}
	if l.profile.Environment == "" {
		result, err := l.queryPreparation(ctx, "runtime_status", "status", "--json")
		if err != nil || result.RuntimeID == "" {
			return "", "", errors.Join(err, errors.New("local Orca runtime identity is unavailable"))
		}
		if l.nativeAccess && !l.structured() {
			if err := l.verifyLocalObservation(ctx, result.RuntimeID); err != nil {
				return "", "", err
			}
		}
		if l.nativeAccess && l.structured() {
			runtime, bootstrapErr := native.LocalRuntimeID()
			if bootstrapErr != nil || runtime != result.RuntimeID {
				return "", "", errors.Join(bootstrapErr, errors.New("local Orca runtime binding changed"))
			}
		}
		return "path:" + repository, result.RuntimeID, nil
	}
	name := repositoryDirectoryName(repository)
	if name == "." || name == "/" || name == "" || strings.HasSuffix(name, ":") {
		return "", "", errors.New("project repository has no directory name")
	}
	result, err := l.queryRepositoryInventory(ctx)
	if err != nil {
		return "", "", fmt.Errorf("cannot query Orca repository inventory for directory %q: %w", name, err)
	}
	listing := native.DecodeRepositoryInventory(result)
	if !listing.Decoded() || result.RuntimeID == "" || listing.Truncated {
		return "", "", errors.New("Orca repository inventory is incomplete")
	}
	for _, repo := range listing.Repos {
		if repo.Kind != "git" || !strings.EqualFold(repositoryDirectoryName(repo.Path), name) {
			continue
		}
		if repo.ID == "" {
			return "", "", errors.New("Orca repository inventory has no repository identity")
		}
		if selector != "" {
			return "", "", fmt.Errorf("multiple Orca Git repositories have directory name %q in environment %q", name, l.profile.Environment)
		}
		selector = "id:" + repo.ID
	}
	if selector == "" {
		return "", "", fmt.Errorf("no Orca Git repository has directory name %q in environment %q; register the matching repository first", name, l.profile.Environment)
	}
	return selector, result.RuntimeID, nil
}

func repositoryDirectoryName(repository string) string {
	return path.Base(strings.TrimRight(strings.ReplaceAll(repository, "\\", "/"), "/"))
}

// Only the known surface-registration race is retryable. The caller supplies
// one deadline shared by inventory and show, and never retries creation/input.
