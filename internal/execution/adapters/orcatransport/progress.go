package orcatransport

import (
	"context"
	"errors"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// No native result/session readiness is consulted. Files remain readable while
// a final is missing and after the original terminal has been stopped.
func ProgressSource(ctx context.Context, setup b.LifecycleSetup, host, instance string) (*native.ProgressFiles, error) {
	profile, _, err := decodeLifecycleProfile(setup)
	if err != nil {
		return nil, errors.New("binding_invalid")
	}
	s := setup.Snapshot
	p := s.Ref.External
	if p.Kind != "orca_terminal" || p.RuntimeID == "" || p.WorktreeID == "" || p.ExecutionHost != profile.Target().ExecutionHost || p.Executable != profile.Executable || s.Ref.Target.ExecutionHost != p.ExecutionHost || s.Ref.WorkspacePath != s.Workspace.Path || s.Session.WorkflowRunID != s.Ref.RunID {
		return nil, errors.New("binding_invalid")
	}
	return native.OpenProgressFiles(ctx, profile.Environment, p.RuntimeID, host, p.WorktreeID, s.Workspace.Path, instance, s.Session.WorkflowRunID)
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
