package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"testing"
	"time"
)

func TestRemoteAuthenticationCapabilityMissingBlocksBeforeBusiness(t *testing.T) {
	l, host, s := fixture(t)
	l.setup.RepositoryURL = "https://github.com/erranliu/SteerlineTest.git"
	_, err := l.PreparationFacts(context.Background(), s)
	if err == nil || !strings.Contains(err.Error(), "remote_sandbox_authentication_unsupported") {
		t.Fatalf("unsupported remote admitted: %v", err)
	}
	if len(host.calls) != 0 {
		t.Fatal("unsupported preparation performed native work")
	}
}

func TestRemoteBootstrapCommitsPreparationWithoutAdditionalControlPrompts(t *testing.T) {
	l, f, s, control := fixtureFixedControl(t)
	l.setup.AgentLaunchCommand = "opaque-start"
	f.commandMode = true
	state, _ := l.state(s)
	controlIntent := l.controlIntent(s, state)
	receipt, _, err := l.Execute(context.Background(), request(s, b.LifecycleCreateResource, b.LifecycleInput{Resource: &controlIntent}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, receipt)
	ready := control.ready
	raw, _ := json.Marshal(ready)
	f.lines = []string{codex.RemoteReadyPrefix + base64.RawURLEncoding.EncodeToString(raw)}
	input := b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Name: "original-terminal", Objective: l.setup.AgentLaunchCommand}}
	receipt, err = l.launchAgent(context.Background(), request(s, b.LifecycleCreateResource, input), false)
	if err != nil {
		t.Fatal(err)
	}
	state, err = decodeReceiptState(l, s, receipt)
	if err != nil || state.RemoteReadiness == nil || state.AdmissionAttempted || f.nativeSends != 0 {
		t.Fatal("preparation was not checkpointed before admission", state, err)
	}
	bind(&s, receipt)
	current := fixtureState(1, "READY", "done")
	current.ProviderSession.ID = ready.SessionID
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
		return &controlledStateFeed{initial: current, current: current}, nil
	}
	calls := len(f.calls)
	receipt, err = l.launchAgent(context.Background(), request(s, b.LifecycleCreateResource, input), true)
	if err != nil || receipt.Disposition != b.OperationConfirmed {
		t.Fatal(receipt, err)
	}
	for _, call := range f.calls[calls:] {
		if len(call) > 1 && call[0] == "terminal" && call[1] == "send" {
			t.Fatal("bootstrap injected a non-business control prompt", call)
		}
	}
	if f.nativeSends != 0 {
		t.Fatal("bootstrap sent business input")
	}
	f.lines = append(f.lines, codex.RemoteFailurePrefix+"authentication_key_unavailable")
	failed, failureErr := l.launchAgent(context.Background(), request(s, b.LifecycleCreateResource, input), true)
	if failureErr == nil || !strings.Contains(failureErr.Error(), "authentication_key_unavailable") || failed.Disposition == b.OperationConfirmed || f.nativeSends != 0 {
		t.Fatal("startup failure after saved readiness admitted business", failed, failureErr)
	}
}

func decodeReceiptState(l *Lifecycle, s b.LifecycleSnapshot, r b.LifecycleReceipt) (State, error) {
	s.Implementation = r.Implementation
	return l.state(s)
}

func TestRemoteReadinessRejectsMissingExpiredAndChangedBindings(t *testing.T) {
	l, _, _ := fixture(t)
	l.setup.RepositoryURL = "https://github.com/erranliu/SteerlineTest.git"
	l.profile.RuntimeExecutable = "C:/runtime/steerrunner-agent.exe"
	l.profile.RuntimeConfigurationDigest = strings.Repeat("a", 64)
	now := time.Now().UTC()
	l.clock = func() time.Time { return now }
	good := codex.RemoteReadiness{ExitPublicKey: make([]byte, 32), Owner: b.ProcessIdentity{PID: 124, Created: "helper-creation"}, NativeTransport: codex.RemoteNativeTransport, Verified: true, Contract: codex.PolicyContract, Repository: l.setup.RepositoryURL, ConfigurationDigest: l.profile.RuntimeConfigurationDigest, SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", Process: b.ProcessIdentity{PID: 123}, CheckedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := l.checkLocalReadiness(State{RemoteReadiness: &good}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []string{"missing", "expired", "wrong-repo", "wrong-config", "wrong-contract", "missing-process", "future", "unverified", "legacy-transport"} {
		t.Run(test, func(t *testing.T) {
			value := good
			state := State{RemoteReadiness: &value}
			switch test {
			case "missing":
				state.RemoteReadiness = nil
			case "expired":
				value.ExpiresAt = now
			case "wrong-repo":
				value.Repository = "https://github.com/other/repo.git"
			case "wrong-config":
				value.ConfigurationDigest = "changed"
			case "wrong-contract":
				value.Contract = "old"
			case "missing-process":
				value.Process.PID = 0
			case "future":
				value.CheckedAt = now.Add(time.Minute)
			case "unverified":
				value.Verified = false
			case "legacy-transport":
				value.NativeTransport = ""
			}
			if err := l.checkLocalReadiness(state); err == nil {
				t.Fatal("invalid remote readiness admitted")
			}
		})
	}
}
