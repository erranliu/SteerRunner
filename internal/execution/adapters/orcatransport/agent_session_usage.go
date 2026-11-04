package orcatransport

import (
	"errors"
	"net/url"
	"strings"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

// StructuredUsageIdentity is an ephemeral view of existing frozen facts. It is
// neither a second session journal nor a fabricated legacy terminal State.
type StructuredUsageIdentity struct {
	RuntimeID, HostID, SessionID, WorktreeID string
}

// Matches requires the public native-session/workspace link first, in addition
// to the exact provider session and CWD checked by the existing source reader.
// "local" here means the paired server's execution host, not the caller host.
func (i StructuredUsageIdentity) Matches(s native.UsageSession) bool {
	return s.ExecutionHostID == "local" && s.StructuredSession != nil && s.StructuredSession.SessionID == i.SessionID && s.StructuredSession.WorkspaceID == i.WorktreeID
}

// StructuredUsageBinding performs no I/O. The caller supplies original frozen
// StageInputs; only their exact digest and original accepted turn can qualify.
func StructuredUsageBinding(profile []b.ImplementationDataEnvelope, snapshot b.LifecycleSnapshot, inputs []b.TurnInput) (parsers.Binding, Profile, StructuredUsageIdentity, error) {
	binding := parsers.Binding{RunID: snapshot.Ref.RunID, AttemptID: snapshot.Ref.AttemptID, Generation: snapshot.ResourceGeneration, Agent: snapshot.Ref.Agent.Name, CWD: snapshot.Ref.WorkspacePath}
	var original StructuredUsageIdentity
	setup := b.LifecycleSetup{Snapshot: snapshot, ProfileData: profile}
	for _, envelope := range profile {
		if envelope.ImplementationKey == AgentSessionProfileKey {
			setup.ProfileID, setup.AcceptedConfigVersion = envelope.ProfileID, envelope.AcceptedConfigVersion
		}
	}
	p, err := decodeAgentSessionProfile(setup)
	if err != nil || binding.RunID == "" || binding.RunID != snapshot.Session.ExecutionID || binding.AttemptID != snapshot.Session.AttemptID || binding.CWD == "" || binding.CWD != snapshot.Workspace.Path {
		return binding, p.Transport, original, errors.New("IDENTITY_UNVERIFIED")
	}
	l := Lifecycle{setup: setup, profile: p.Transport}
	state, err := l.agentState(snapshot)
	if err != nil {
		return binding, p.Transport, original, errors.New("IDENTITY_CHANGED")
	}
	if state.TurnID == "" || state.TurnItemID == "" || state.ProviderItemID == "" || state.Epoch == "" {
		return binding, p.Transport, original, errors.New("SESSION_UNBOUND")
	}
	matched := 0
	for _, input := range inputs {
		if input.CommandID == state.CommandID {
			if digest([]byte(input.Prompt)) != state.InputDigest {
				return binding, p.Transport, original, errors.New("IDENTITY_CHANGED")
			}
			matched++
			binding.Commands = append(binding.Commands, parsers.Command{ID: input.CommandID, WireSHA256: state.InputDigest, NativeTurnID: state.TurnID})
		}
	}
	if matched != 1 {
		return binding, p.Transport, original, errors.New("SESSION_UNBOUND")
	}
	// Cross-check only the known public Codex item identity shape. Do not guess
	// unknown opaque formats or treat this as a replacement for the direct link.
	parts := strings.Split(state.ProviderItemID, ":")
	if len(parts) != 4 || parts[0] != "codex" || parts[3] != "0" {
		return binding, p.Transport, original, errors.New("IDENTITY_UNVERIFIED")
	}
	provider, providerErr := url.PathUnescape(parts[1])
	turn, turnErr := url.PathUnescape(parts[2])
	if providerErr != nil || turnErr != nil || !usageSessionID.MatchString(provider) || turn != state.TurnID {
		return binding, p.Transport, original, errors.New("IDENTITY_CHANGED")
	}
	binding.SessionID = provider
	original = StructuredUsageIdentity{RuntimeID: state.RuntimeID, HostID: p.PairingHostID, SessionID: state.SessionID, WorktreeID: state.Worktree.ID}
	if p.Transport.Environment == "" {
		original.HostID = "local"
	}
	return binding, p.Transport, original, nil
}
