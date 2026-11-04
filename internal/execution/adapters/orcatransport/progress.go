package orcatransport

import (
	"context"
	"errors"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

// Routing does not consult native result/session readiness. Progress can read
// while a final is missing; result callers open only after execution ends. Both
// use the original worktree, which outlives the original terminal.
func ProgressSource(ctx context.Context, setup b.LifecycleSetup, host, instance string) (*native.ScopedFiles, error) {
	return fileSource(ctx, setup, host, instance, false)
}
func ResultSource(ctx context.Context, setup b.LifecycleSetup, host, instance string) (*native.ScopedFiles, error) {
	return fileSource(ctx, setup, host, instance, true)
}
func fileSource(ctx context.Context, setup b.LifecycleSetup, host, instance string, result bool) (*native.ScopedFiles, error) {
	profile, _, err := decodeLifecycleProfile(setup)
	if err != nil {
		return nil, errors.New("binding_invalid")
	}
	s := setup.Snapshot
	p := s.Ref.External
	kind := "orca_terminal"
	if s.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 {
		kind = "orca_agent_session"
		l := &Lifecycle{setup: setup, profile: profile}
		state, stateErr := l.agentState(s)
		if stateErr != nil || state.Worktree.Path != s.Workspace.Path || instance != "" && instance != state.Worktree.InstanceID {
			return nil, errors.New("binding_invalid")
		}
		instance = state.Worktree.InstanceID
	}
	if p.Kind != kind || p.RuntimeID == "" || p.WorktreeID == "" || p.ExecutionHost != profile.Target().ExecutionHost || p.Executable != profile.Executable || s.Ref.Target.ExecutionHost != p.ExecutionHost || s.Ref.WorkspacePath != s.Workspace.Path || s.Session.WorkflowRunID != s.Ref.RunID {
		return nil, errors.New("binding_invalid")
	}
	return openRunFile(ctx, profile.Environment, p.RuntimeID, host, p.WorktreeID, s.Workspace.Path, instance, s.Session.WorkflowRunID, result)
}

const runtimeFileLimit = 65536

func openRunFile(ctx context.Context, environment, runtime, host, worktree, workspace, instance, run string, result bool) (*native.ScopedFiles, error) {
	if len(run) != 32 || strings.ContainsAny(run, "/\\.:\x00\r\n") {
		return nil, errors.New("binding_invalid")
	}
	root, name := ".steerrunner/progress/"+run, "progress.bin"
	if result {
		root, name = ".steerrunner/result/"+run, "result.bin"
	}
	return native.OpenWorkspaceFiles(ctx, native.WorkspaceFileBinding{Environment: environment, RuntimeID: runtime, ExecutionHostID: host, WorktreeID: worktree, WorkspacePath: workspace, InstanceID: instance}, native.FileScope{Root: root, RelativePath: root + "/" + name}, native.FileLimits{ReadBytes: runtimeFileLimit, WriteBytes: runtimeFileLimit})
}

func ProgressHost(setup b.LifecycleSetup) (string, error) {
	p, _, err := decodeLifecycleProfile(setup)
	if err != nil {
		return "", err
	}
	if p.Environment == "" {
		return "local", nil
	}
	if setup.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 {
		profile, err := decodeAgentSessionProfile(setup)
		if err != nil {
			return "", err
		}
		return profile.PairingHostID, nil
	}
	return native.UsageHostID(p.Environment)
}
