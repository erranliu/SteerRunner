package application

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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
		_, err := codex.DecodePolicyResourceEnvelope(*s.PolicyResources, identity)
		return err
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
