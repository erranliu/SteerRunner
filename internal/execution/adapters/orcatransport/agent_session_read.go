package orcatransport

import (
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Shared read validation owns no host, process probe, observer or mutable state.
// Development is explicit: a queued next input may coexist with the prior turn.
func validateStructuredSnapshot(original b.LifecycleSnapshot, profile Profile, s b.LifecycleSnapshot) error {
	if err := validateSnapshotBinding(original, profile, s); err != nil {
		return err
	}
	if s.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || s.Ref.Agent.Name != "codex" && s.Ref.Agent.Name != "pi" {
		return errors.New("unsupported structured Orca identity")
	}
	return nil
}

func readAgentSessionState(original b.LifecycleSnapshot, profile Profile, development bool, s b.LifecycleSnapshot) (agentSessionState, error) {
	var state agentSessionState
	if err := validateStructuredSnapshot(original, profile, s); err != nil {
		return state, err
	}
	found := false
	for _, e := range s.Implementation {
		if e.ImplementationKey != AgentSessionStateKey {
			continue
		}
		if found || e.CodecVersion != AgentSessionVersion {
			return state, errors.New("invalid structured Orca state codec")
		}
		found = true
		if err := e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest); err != nil {
			return state, err
		}
		if err := b.DecodeStrictJSON(e.Payload, &state, "trailing structured Orca state"); err != nil {
			return state, err
		}
	}
	if found && (state.RuntimeID == "" || state.Worktree.ID == "" || state.Worktree.InstanceID == "" || state.SessionID != agentSessionID(s) || state.Worktree.ID != s.Ref.External.WorktreeID || state.RuntimeID != s.Ref.External.RuntimeID || s.Ref.External.Kind != "orca_agent_session" || s.Ref.External.Executable != profile.Executable || s.Ref.External.ExecutionHost != profile.Target().ExecutionHost || s.Ref.External.Terminal != "" || s.Ref.External.PID != 0 || s.Ref.External.Incarnation != "" || state.Worktree.Path != s.Workspace.Path) {
		return state, errors.New("original structured Orca resource binding changed")
	}
	if len(state.CloseReceipt) > 0 && (!state.CloseAttempted || !agentCloseSucceeded(state.CloseReceipt)) {
		return state, errors.New("original structured close receipt is invalid")
	}
	if state.SessionNotCreated && agentSessionMayExist(s, state) {
		return state, errors.New("structured session absence contradicts its original dispatch")
	}
	if state.CommandID != "" && (!b.StableExecutionID(state.CommandID) || len(state.InputDigest) != 64 || len(state.BodyFingerprint) != 64 || state.CreateID == "" || state.MessageID == "") {
		return state, errors.New("incomplete original structured input binding")
	}
	if s.PendingTurn != nil && state.CommandID != "" && (state.CommandID == s.PendingTurn.CommandID && state.InputDigest != digest([]byte(s.PendingTurn.Prompt)) || !development && state.CommandID != s.PendingTurn.CommandID) {
		return state, errors.New("original structured input changed")
	}
	return state, nil
}

func validateSnapshotBinding(original b.LifecycleSnapshot, profile Profile, s b.LifecycleSnapshot) error {
	w := original
	if s.Ref.Backend != w.Ref.Backend || s.Ref.Protocol != w.Ref.Protocol || s.Ref.Target.ID != w.Ref.Target.ID || s.Ref.Agent != w.Ref.Agent || s.Ref.RunID != w.Ref.RunID || s.Ref.AttemptID != w.Ref.AttemptID || s.Session.ProjectID != w.Session.ProjectID || s.Session.WorkflowRunID != w.Session.WorkflowRunID || s.Ref.Target.ExternalRuntime != profile.Target().ExternalRuntime || s.Ref.Target.ExecutionHost != profile.Target().ExecutionHost || s.Session.ExecutionID != w.Session.ExecutionID || s.Session.AttemptID != w.Session.AttemptID || s.ConfigurationDigest != w.ConfigurationDigest || s.ResourceGeneration != w.ResourceGeneration {
		return errors.New("Orca terminal execution binding changed")
	}
	if s.Session.ProjectID == "" || s.ConfigurationDigest == "" || s.ResourceGeneration == "" {
		return errors.New("incomplete Orca terminal execution")
	}
	return nil
}
