package application

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func (e *Engine) runsPolicyMissing(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runs[id].Snapshot.PolicyResources == nil
}

// Qualification is committed before session creation. Reconciliation of an
// existing native effect never fabricates resources for an unqualified session.
func (e *Engine) verifyPolicyResources(r *runstate.Record) error {
	s := &r.Snapshot
	identity := codex.PolicyResourceIdentity{ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}
	if s.PolicyResources != nil {
		payload, err := codex.DecodePolicyResourceEnvelope(*s.PolicyResources, identity)
		if err != nil {
			return err
		}
		profile, err := codex.DecodeProfileConfigEnvelope(r.Profile, codex.ProfileConfigIdentity{ProjectID: s.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest})
		if err != nil {
			return err
		}
		mode := profile.AccessMode
		if mode == "" {
			mode = "sandbox"
		}
		actual := "sandbox"
		for _, arg := range payload.Settings.Args {
			if arg == "sandbox_mode=danger-full-access" {
				actual = "full_access"
			}
		}
		if actual != mode {
			return errors.New("permission resources differ from frozen access mode")
		}
		return nil
	}
	if s.Ref.External.Kind != "" || s.Session.SessionID != "" && s.Session.AttemptNumber <= 1 {
		return errors.New("original session has no verified permission resources; preserve it without new input")
	}
	if err := e.attachWriteRoots(r); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()
	bound, err := codex.BuildPolicyResourceEnvelope(ctx, codex.PolicyResourceBuildRequest{
		ProfileData: r.Profile, ProfileIdentity: codex.ProfileConfigIdentity{ProjectID: s.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest},
		ResourceIdentity: identity, Host: "local", Agent: "codex", GOOS: runtime.GOOS, Workspace: s.Workspace.Path, LinkedWriteArgs: r.LinkedWriteArgs,
		RunDirectory: e.dir, RepositoryURL: r.Request.Repository.URL,
	}, func(ctx context.Context, executable string) (string, error) {
		out, err := (commandHost{}).Output(ctx, b.Command{Executable: executable}, "--version")
		return strings.TrimSpace(string(out)), err
	})
	if err != nil {
		return err
	}
	if _, err := codex.DecodePolicyResourceEnvelope(bound.Envelope, identity); err != nil {
		return err
	}
	s.PolicyResources = &bound.Envelope
	return nil
}

func (e *Engine) attachWriteRoots(r *runstate.Record) error {
	if r.Request.Execution.Adapter != "codex-app-server" {
		return nil
	}
	roots, err := workspace.ResolveLinkedResources(e.ctx, gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, &r.Snapshot.Workspace)
	if err != nil {
		return err
	}
	args, err := codex.LinkedWorktreeWriteArgs(roots.Objects, roots.Refs, roots.Logs, roots.Admin)
	if err != nil {
		return err
	}
	r.LinkedWriteArgs = args
	if r.Snapshot.PendingTurn != nil {
		r.Snapshot.PendingTurn.LinkedWorktreeArgs = args
	}
	return nil
}
