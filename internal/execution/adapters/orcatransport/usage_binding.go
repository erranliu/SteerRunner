package orcatransport

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

var usageSessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// Historical terminal launches used this plain PowerShell invocation. Restrict
// recognition to its literal grammar: a substring in shell code is not proof
// that an initialization prompt was delivered.
var usagePlainLaunch = regexp.MustCompile(`^& '(?:[^']|'')*'(?: '(?:[^']|'')*')*$`)

const usageReadyPrompt = codex.BootstrapInstruction

type UsageInitialization struct {
	LaunchCommand string
	CreatedAt     time.Time
	RunID         string
}

func usageWirePrompt(turn *Turn, agent string) string {
	wire := wirePrompt(turn)
	if agent == "codex" && turn.WireFormat == "" {
		// The historical TUI removes final line terminators. Spaces and every
		// interior byte remain significant; this is not fuzzy prompt matching.
		wire = strings.TrimRight(strings.ReplaceAll(wire, "\r\n", "\n"), "\r\n")
	}
	return wire
}

func usageInitialization(state State, proof UsageInitialization, profiles ...Profile) *parsers.Command {
	if !state.LaunchAccepted || !state.LaunchConfirmed || proof.CreatedAt.IsZero() || digest([]byte(proof.LaunchCommand)) != state.LaunchDigest {
		return nil
	}
	prompt := usageReadyPrompt
	launch := proof.LaunchCommand
	if inner, ok := agentlaunch.UnwrapFileCommands(launch, proof.RunID); ok {
		launch = inner
	}
	if inner, ok := agentlaunch.UnwrapTeaPathLaunch(launch); ok {
		launch = inner
	}
	plain := usagePlainLaunch.MatchString(launch) && strings.HasSuffix(launch, " '"+prompt+"'")
	if !plain && usagePlainLaunch.MatchString(launch) && strings.HasSuffix(launch, " '"+codex.NativeReadinessPrompt+"'") {
		prompt, plain = codex.NativeReadinessPrompt, true
	}
	preparedProof := proof
	preparedProof.LaunchCommand = launch
	if !plain && (len(profiles) != 1 || !usagePreparedInitialization(state, preparedProof, profiles[0])) {
		return nil
	}
	var until int64
	for _, turn := range state.Turns {
		if turn.Accepted && turn.Native.BaseRootAt > 0 && (until == 0 || turn.Native.BaseRootAt < until) {
			until = turn.Native.BaseRootAt
		}
	}
	end := time.UnixMilli(until).UTC()
	if until == 0 || !end.After(proof.CreatedAt) {
		return nil
	}
	return &parsers.Command{ID: "$initialization", Phase: "initialization", WireSHA256: parsers.TextHash(prompt), NotBefore: &proof.CreatedAt, NotAfter: &end}
}

// Existing local execution runtime: only its fixed, configuration-bound native
// bootstrap qualifies. Arbitrary wrappers remain unsupported. This reads
// committed evidence; it never starts, changes or deploys that runtime.
func usagePreparedInitialization(state State, proof UsageInitialization, p Profile) bool {
	r := state.RemoteReadiness
	if p.Environment != "" || p.Terminal != nil || p.RuntimeExecutable == "" || len(p.RuntimeSHA256) != 64 || r == nil || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.RuntimeSHA256 != p.RuntimeSHA256 || r.ConfigurationDigest == "" || r.ConfigurationDigest != p.RuntimeConfigurationDigest || !usageSessionID.MatchString(r.SessionID) || r.Workspace == "" || r.Owner.Kind != "os" || r.Owner.PID <= 0 || r.Owner.Created == "" || r.Process.Kind != "os" || r.Process.PID <= 0 || r.Process.Created == "" || r.CheckedAt.Before(proof.CreatedAt) {
		return false
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	prefix := "& " + quote(p.RuntimeExecutable) + " '--configuration' '"
	if !strings.HasPrefix(proof.LaunchCommand, prefix) || !strings.HasSuffix(proof.LaunchCommand, "'") {
		return false
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(proof.LaunchCommand, prefix), "'")
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	var config codex.RemoteAgentConfig
	if b.DecodeStrictJSON(raw, &config, "trailing configuration") != nil || codex.RemoteConfigurationDigest(config) != p.RuntimeConfigurationDigest || config.RepositoryURL == "" || config.RepositoryURL != r.Repository {
		return false
	}
	matched := false
	for _, turn := range state.Turns {
		if turn.Accepted {
			if turn.Native.Provider != "" && turn.Native.Provider != r.SessionID {
				return false
			}
			matched = matched || turn.Native.Provider == r.SessionID && turn.Native.BaseRootAt > 0
		}
	}
	return matched
}

// UsageInputDigest is independent of the current launch profile. Completed old
// generations retain their history after a later generation changes that profile.
func UsageInputDigest(s b.LifecycleSnapshot) string {
	type input struct {
		Command, Wire, Session      string
		Accepted, Settled, Rejected bool
		BaseRootAt, EndedAt         int64
		Recorded                    any
		Extension                   any
	}
	var inputs []input
	var readiness []any
	for _, envelope := range s.Implementation {
		if envelope.ImplementationKey != StateKey {
			continue
		}
		if envelope.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest) != nil {
			return ""
		}
		var state State
		if json.Unmarshal(envelope.Payload, &state) != nil {
			return ""
		}
		for _, turn := range state.Turns {
			inputs = append(inputs, input{turn.Command, wirePrompt(&turn), turn.Native.Provider, turn.Accepted, turn.Settled, turn.Rejected, turn.Native.BaseRootAt, turn.Native.EndedAt, turn.Recorded, turn.Extension})
		}
		readiness = append(readiness, state.RemoteReadiness, state.TerminalReadiness)
	}
	raw, _ := json.Marshal(struct {
		Inputs                    []input
		Readiness                 []any
		Ref                       b.ExecutionRef
		Session                   b.SessionContext
		Configuration, Generation string
		Pending                   *b.TurnInput
	}{inputs, readiness, s.Ref, s.Session, s.ConfigurationDigest, s.ResourceGeneration, s.PendingTurn})
	h := sha256.Sum256(raw)
	return fmt.Sprintf("%x", h)
}

// UsageBinding only exports already committed execution evidence. It cannot
// prepare a session, inspect an agent, or send input. Paths remain host-private.
func UsageBinding(profile []b.ImplementationDataEnvelope, s b.LifecycleSnapshot, initialization ...UsageInitialization) (parsers.Binding, Profile, State, error) {
	binding := parsers.Binding{RunID: s.Ref.RunID, AttemptID: s.Ref.AttemptID, Generation: s.ResourceGeneration, Agent: s.Ref.Agent.Name, CWD: s.Ref.WorkspacePath}
	var p Profile
	if binding.RunID == "" || binding.RunID != s.Session.ExecutionID || binding.AttemptID != s.Session.AttemptID {
		return binding, p, State{}, errors.New("IDENTITY_UNVERIFIED")
	}
	found := false
	for _, e := range profile {
		if e.ImplementationKey == ProfileKey {
			if e.CodecVersion != ProfileVersion && e.CodecVersion != NativeRemoteProfileVersion && e.CodecVersion != FullAccessTerminalProfileVersion {
				return binding, p, State{}, errors.New("FORMAT_UNSUPPORTED")
			}
			if e.Validate() != nil {
				return binding, p, State{}, errors.New("IDENTITY_UNVERIFIED")
			}
			var err error
			p, err = DecodeProfile(e.Payload)
			if err != nil {
				return binding, p, State{}, err
			}
			found = true
		}
	}
	if !found {
		return binding, p, State{}, errors.New("FORMAT_UNSUPPORTED")
	}
	if p.Environment != "" && binding.Agent != "codex" {
		return binding, p, State{}, errors.New("PRODUCTION_BINDING_UNSUPPORTED")
	}
	version := LocalStateVersion
	if p.Environment != "" {
		version = StateVersion
	}
	var state State
	found = false
	for _, e := range s.Implementation {
		if e.ImplementationKey == StateKey {
			if e.CodecVersion != version || e.ValidateResourceIdentity(s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest) != nil {
				return binding, p, state, errors.New("IDENTITY_UNVERIFIED")
			}
			var err error
			state, err = decodeState(e.Payload, e.CodecVersion)
			if err != nil {
				return binding, p, state, errors.New("FORMAT_UNSUPPORTED")
			}
			found = true
		}
	}
	if !found {
		return binding, p, state, errors.New("SESSION_UNBOUND")
	}
	if p.Terminal != nil {
		if p.Terminal.ExecutionID != binding.RunID || p.Terminal.AttemptID != binding.AttemptID || p.Terminal.Generation != binding.Generation {
			return binding, p, state, errors.New("IDENTITY_CHANGED")
		}
		binding.SessionID = p.Terminal.SessionID
		if binding.Agent == "dsh" && state.TerminalReadiness != nil {
			binding.SessionID = state.TerminalReadiness.NativeSessionID
		}
	} else if state.RemoteReadiness != nil {
		binding.SessionID = state.RemoteReadiness.SessionID
		if state.RemoteReadiness.Workspace != "" && parsers.NormalizePath(state.RemoteReadiness.Workspace) != parsers.NormalizePath(binding.CWD) {
			return binding, p, state, errors.New("IDENTITY_CHANGED")
		}
	}
	for _, turn := range state.Turns {
		if !turn.Accepted {
			continue
		}
		if binding.SessionID == "" {
			binding.SessionID = turn.Native.Provider
		}
		if turn.Native.Provider != "" && turn.Native.Provider != binding.SessionID {
			return binding, p, state, errors.New("SESSION_AMBIGUOUS")
		}
		if turn.InputDigest != "" && turn.InputDigest != digest([]byte(turn.Prompt)) {
			return binding, p, state, errors.New("IDENTITY_CHANGED")
		}
		c := parsers.Command{ID: turn.Command, WireSHA256: parsers.TextHash(usageWirePrompt(&turn, binding.Agent))}
		if binding.Agent == "codex" && turn.Native.BaseRootAt > 0 {
			start := time.UnixMilli(turn.Native.BaseRootAt).UTC()
			c.NotBefore = &start
			if turn.Native.EndedAt > 0 {
				end := time.UnixMilli(turn.Native.EndedAt).UTC()
				if !end.After(start) {
					return binding, p, state, errors.New("IDENTITY_CHANGED")
				}
				c.NotAfter = &end
			}
		}
		if turn.Recorded != nil {
			if turn.Recorded.InputDigest != c.WireSHA256 || turn.Recorded.NativeTurnID == "" || turn.Recorded.UserItemID == "" {
				return binding, p, state, errors.New("IDENTITY_CHANGED")
			}
			c.NativeTurnID = turn.Recorded.NativeTurnID
			// Codex thread/read synthesizes user item IDs in a different identity
			// space from rollout response_item.id. The receipt proves the native
			// turn and exact wire digest; its item ID is not a rollout locator.
		}
		if receipt := turn.Extension; receipt != nil {
			if receipt.WireSHA256 != c.WireSHA256 || receipt.RunID != binding.RunID || p.Terminal == nil || receipt.SessionID != p.Terminal.SessionID {
				return binding, p, state, errors.New("IDENTITY_CHANGED")
			}
			c.InputID = receipt.NativeInputID
			c.ReplyID = receipt.NativeReplyID
			c.InputSeq = int64(receipt.NativeInputSeq)
			c.EndSeq = int64(receipt.NativeEndSeq)
		}
		binding.Commands = append(binding.Commands, c)
	}
	if binding.Agent == "codex" && len(initialization) == 1 {
		proof := initialization[0]
		proof.RunID = binding.RunID
		if command := usageInitialization(state, proof, p); command != nil {
			binding.Commands = append(binding.Commands, *command)
		}
	}
	if binding.SessionID == "" || binding.CWD == "" || len(binding.Commands) == 0 {
		return binding, p, state, errors.New("SESSION_UNBOUND")
	}
	if !usageSessionID.MatchString(binding.SessionID) {
		return binding, p, state, errors.New("IDENTITY_UNVERIFIED")
	}
	return binding, p, state, nil
}
