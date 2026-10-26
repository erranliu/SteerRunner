package orcatransport

import (
	"context"
	"errors"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Routing does not consult native result/session readiness. Progress can read
// while a final is missing; result callers open only after execution ends. Both
// use the original worktree, which outlives the original terminal.
func ProgressSource(ctx context.Context, setup b.LifecycleSetup, host, instance string) (*native.ProgressFiles, error) {
	return fileSource(ctx, setup, host, instance, false)
}
func ResultSource(ctx context.Context, setup b.LifecycleSetup, host, instance string) (*native.ProgressFiles, error) {
	return fileSource(ctx, setup, host, instance, true)
}
func fileSource(ctx context.Context, setup b.LifecycleSetup, host, instance string, result bool) (*native.ProgressFiles, error) {
	profile, _, err := decodeLifecycleProfile(setup)
	if err != nil {
		return nil, errors.New("binding_invalid")
	}
	s := setup.Snapshot
	p := s.Ref.External
	if p.Kind != "orca_terminal" || p.RuntimeID == "" || p.WorktreeID == "" || p.ExecutionHost != profile.Target().ExecutionHost || p.Executable != profile.Executable || s.Ref.Target.ExecutionHost != p.ExecutionHost || s.Ref.WorkspacePath != s.Workspace.Path || s.Session.WorkflowRunID != s.Ref.RunID {
		return nil, errors.New("binding_invalid")
	}
	open := native.OpenProgressFiles
	if result {
		open = native.OpenResultFiles
	}
	return open(ctx, profile.Environment, p.RuntimeID, host, p.WorktreeID, s.Workspace.Path, instance, s.Session.WorkflowRunID)
}
func ProgressHost(setup b.LifecycleSetup) (string, error) {
	p, _, err := decodeLifecycleProfile(setup)
	if err != nil {
		return "", err
	}
	if p.Environment == "" {
		return "local", nil
	}
	return native.UsageHostID(p.Environment)
}
