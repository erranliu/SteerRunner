package orcatransport

import (
	"encoding/base64"
	"encoding/json"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
	"strings"
	"testing"
	"time"
)

func TestUsagePreparedLocalCodexInitialization(t *testing.T) {
	start := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	c := codex.RemoteAgentConfig{Executable: "native-codex.exe", RepositoryURL: "https://github.com/owner/repo.git", AccessMode: "sandbox"}
	raw, _ := json.Marshal(c)
	p := Profile{RuntimeExecutable: `C:\existing\agent.exe`, RuntimeSHA256: strings.Repeat("a", 64), RuntimeConfigurationDigest: codex.RemoteConfigurationDigest(c)}
	launch := `& 'C:\existing\agent.exe' '--configuration' '` + base64.RawURLEncoding.EncodeToString(raw) + `'`
	proof := UsageInitialization{LaunchCommand: launch, CreatedAt: start}
	state := State{LaunchAccepted: true, LaunchConfirmed: true, LaunchDigest: digest([]byte(launch)), RemoteReadiness: &codex.RemoteReadiness{Verified: true, NativeTransport: codex.RemoteNativeTransport, Contract: codex.PolicyContract, RuntimeSHA256: p.RuntimeSHA256, ConfigurationDigest: p.RuntimeConfigurationDigest, SessionID: "native-session", Workspace: `C:\work`, Repository: c.RepositoryURL, Owner: b.ProcessIdentity{Kind: "os", PID: 1, Created: "owner"}, Process: b.ProcessIdentity{Kind: "os", PID: 2, Created: "process"}, CheckedAt: start.Add(time.Second)}, Turns: []Turn{{Accepted: true, Native: NativeProof{Provider: "native-session", BaseRootAt: start.Add(time.Minute).UnixMilli()}}}}
	if got := usageInitialization(state, proof, p); got == nil || got.Phase != "initialization" || got.ID != "$initialization" {
		t.Fatal("verified fixed bootstrap omitted", got)
	}
	for _, mutate := range []func(*State, *UsageInitialization, *Profile){
		func(s *State, _ *UsageInitialization, _ *Profile) { s.RemoteReadiness.Verified = false },
		func(s *State, _ *UsageInitialization, _ *Profile) { s.RemoteReadiness.RuntimeSHA256 = "different" },
		func(s *State, _ *UsageInitialization, _ *Profile) {
			s.RemoteReadiness.ConfigurationDigest = "different"
		},
		func(s *State, _ *UsageInitialization, _ *Profile) { s.RemoteReadiness.Contract = "unknown" },
		func(s *State, _ *UsageInitialization, _ *Profile) { s.RemoteReadiness.Owner.Created = "" },
		func(s *State, _ *UsageInitialization, _ *Profile) { s.Turns[0].Native.Provider = "another-session" },
		func(_ *State, _ *UsageInitialization, p *Profile) { p.Environment = "remote" },
		func(s *State, proof *UsageInitialization, _ *Profile) {
			proof.LaunchCommand += "; Write-Output 'other'"
			s.LaunchDigest = digest([]byte(proof.LaunchCommand))
		},
		func(s *State, proof *UsageInitialization, _ *Profile) {
			proof.LaunchCommand = strings.ReplaceAll(proof.LaunchCommand, "agent.exe", "unknown.exe")
			s.LaunchDigest = digest([]byte(proof.LaunchCommand))
		},
	} {
		encoded, _ := json.Marshal(state)
		var clone State
		_ = json.Unmarshal(encoded, &clone)
		pf, pr := p, proof
		mutate(&clone, &pr, &pf)
		if usageInitialization(clone, pr, pf) != nil {
			t.Fatal("unverified bootstrap admitted")
		}
	}
}

func TestUsageRecordedCodexIDsUseOriginalTurnAndWireDigest(t *testing.T) {
	snapshot := b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt", WorkspacePath: `C:\work`, Agent: b.AgentSpec{Name: "codex"}}, Session: b.SessionContext{ProjectID: "project", ExecutionID: "run", AttemptID: "attempt"}, ResourceGeneration: "generation", ConfigurationDigest: "configuration"}
	raw, _ := json.Marshal(Profile{Executable: "orca", WaitTimeout: "1s"})
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, CodecVersion: ProfileVersion, ProjectID: "project", ProfileID: "profile", AcceptedConfigVersion: "1"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	state := State{Turns: []Turn{{Command: "command", Accepted: true, Prompt: "exact input", InputDigest: digest([]byte("exact input")), Native: NativeProof{Provider: "native-session"}, Recorded: &codex.RemoteTurnReceipt{NativeTurnID: "native-turn", UserItemID: "thread-read-item-not-rollout-item", InputDigest: digest([]byte("exact input"))}}}}
	bind := func() (parsers.Binding, error) {
		raw, _ := json.Marshal(state)
		envelope, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{Purpose: b.ImplementationResourcePurpose, ImplementationKey: StateKey, CodecVersion: StateVersion, ProjectID: "project", ExecutionID: "run", AttemptID: "attempt", Generation: "generation", ConfigDigest: "configuration"}, raw)
		if err != nil {
			t.Fatal(err)
		}
		snapshot.Implementation = []b.ImplementationDataEnvelope{envelope}
		binding, _, _, err := UsageBinding([]b.ImplementationDataEnvelope{profile}, snapshot)
		return binding, err
	}
	got, err := bind()
	if err != nil || len(got.Commands) != 1 || got.Commands[0].NativeTurnID != "native-turn" || got.Commands[0].InputID != "" {
		t.Fatal(got, err)
	}
	state.Turns[0].Recorded.InputDigest = "different"
	if _, err = bind(); err == nil || err.Error() != "IDENTITY_CHANGED" {
		t.Fatal("foreign receipt digest accepted", err)
	}
	state.Turns[0].Recorded.InputDigest = digest([]byte("exact input"))
	state.RemoteReadiness = &codex.RemoteReadiness{SessionID: "native-session", Workspace: `D:\other`}
	if _, err = bind(); err == nil || err.Error() != "IDENTITY_CHANGED" {
		t.Fatal("foreign readiness workspace accepted", err)
	}
}

func TestUsageHistoricalCodexWireAndInitialization(t *testing.T) {
	turn := Turn{Prompt: "exact body \r\n\n"}
	if got := usageWirePrompt(&turn, "codex"); got != "exact body " {
		t.Fatal(got)
	}
	if got := usageWirePrompt(&turn, "pi"); got != turn.Prompt {
		t.Fatal("changed Pi wire")
	}
	start := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	launch := "& 'codex.exe' '--model' 'observed-at-launch' '" + usageReadyPrompt + "'"
	proof := UsageInitialization{LaunchCommand: launch, CreatedAt: start}
	state := State{LaunchAccepted: true, LaunchConfirmed: true, LaunchDigest: digest([]byte(launch)), Turns: []Turn{{Accepted: true, Native: NativeProof{BaseRootAt: start.Add(time.Minute).UnixMilli()}}}}
	got := usageInitialization(state, proof)
	if got == nil || got.Phase != "initialization" || !got.NotBefore.Equal(start) || !got.NotAfter.Equal(start.Add(time.Minute)) {
		t.Fatal(got)
	}
	for _, mutate := range []func(*State, *UsageInitialization){
		func(s *State, p *UsageInitialization) { s.LaunchAccepted = false },
		func(s *State, p *UsageInitialization) { s.LaunchDigest = "unverified" },
		func(s *State, p *UsageInitialization) { p.CreatedAt = time.Time{} },
		func(s *State, p *UsageInitialization) { p.CreatedAt = start.Add(2 * time.Minute) },
		func(s *State, p *UsageInitialization) {
			p.LaunchCommand = launch + "; Write-Output 'ignored'"
			s.LaunchDigest = digest([]byte(p.LaunchCommand))
		},
		func(s *State, p *UsageInitialization) {
			p.LaunchCommand = "Write-Output '" + usageReadyPrompt + "'"
			s.LaunchDigest = digest([]byte(p.LaunchCommand))
		},
	} {
		s, p := state, proof
		mutate(&s, &p)
		if usageInitialization(s, p) != nil {
			t.Fatal("unverified initialization attributed")
		}
	}
}

func TestUsageScopedReadinessUsesItsOwnWireDigest(t *testing.T) {
	start := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	for _, prompt := range []string{usageReadyPrompt, codex.NativeReadinessPrompt} {
		launch := "& 'codex.exe' '--model' 'chosen' '" + prompt + "'"
		proof := UsageInitialization{LaunchCommand: launch, CreatedAt: start}
		state := State{LaunchAccepted: true, LaunchConfirmed: true, LaunchDigest: digest([]byte(launch)), Turns: []Turn{{Accepted: true, Native: NativeProof{BaseRootAt: start.Add(time.Minute).UnixMilli()}}}}
		got := usageInitialization(state, proof)
		if got == nil || got.WireSHA256 != parsers.TextHash(prompt) {
			t.Fatal("initialization attributed to a different wire input", got)
		}
		proof.LaunchCommand += "; Write-Output 'READY'"
		state.LaunchDigest = digest([]byte(proof.LaunchCommand))
		if got := usageInitialization(state, proof); got != nil {
			t.Fatal("arbitrary command qualified as initialization", got)
		}
	}
}

func TestUsageTeaWrappedCodexInitialization(t *testing.T) {
	start := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	c := &agentlaunch.Config{Executable: "C:/native/codex.exe", Model: "chosen", AccessMode: "full_access", RepositoryURL: "http://gitea/owner/repo.git"}
	launch, err := c.Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	proof := UsageInitialization{LaunchCommand: launch, CreatedAt: start}
	state := State{LaunchAccepted: true, LaunchConfirmed: true, LaunchDigest: digest([]byte(launch)), Turns: []Turn{{Accepted: true, Native: NativeProof{BaseRootAt: start.Add(time.Minute).UnixMilli()}}}}
	got := usageInitialization(state, proof)
	if got == nil || got.WireSHA256 != parsers.TextHash(codex.NativeReadinessPrompt) || got.Phase != "initialization" {
		t.Fatal("Runner's own publication PATH wrapper lost initialization usage", got)
	}
	for _, bad := range []string{
		launch + "; Write-Output 'other'",
		"Write-Output 'other'; " + launch,
		strings.Replace(launch, "$env:PATH=$srTeaEntries -join ';'", "$env:PATH='other'", 1),
		strings.Replace(launch, "& 'C:/native/codex.exe'", "Write-Output 'not an agent'; & 'C:/native/codex.exe'", 1),
		strings.Replace(launch, codex.NativeReadinessPrompt, usageReadyPrompt+" other", 1),
	} {
		p, s := proof, state
		p.LaunchCommand, s.LaunchDigest = bad, digest([]byte(bad))
		if usageInitialization(s, p) != nil {
			t.Fatal("modified or arbitrary wrapper attributed")
		}
	}
	state.LaunchDigest = "different"
	if usageInitialization(state, proof) != nil {
		t.Fatal("unverified full wrapper digest attributed")
	}
}

func TestUsageProgressAndShellWrappedInitialization(t *testing.T) {
	start := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	run := strings.Repeat("a", 32)
	c := &agentlaunch.Config{Executable: "C:/native/codex.exe", Model: "chosen", AccessMode: "full_access", RepositoryURL: "http://gitea/owner/repo.git"}
	inner, err := c.Command("codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, progressRun := range []string{"", run} {
		launch := agentlaunch.ProgressCommand(inner, progressRun)
		proof := UsageInitialization{LaunchCommand: launch, CreatedAt: start, RunID: run}
		state := State{LaunchAccepted: true, LaunchConfirmed: true, LaunchDigest: digest([]byte(launch)), Turns: []Turn{{Accepted: true, Native: NativeProof{BaseRootAt: start.Add(time.Minute).UnixMilli()}}}}
		if got := usageInitialization(state, proof); got == nil || got.WireSHA256 != parsers.TextHash(codex.NativeReadinessPrompt) {
			t.Fatal("progress launch lost initialization", progressRun, got)
		}
		for _, bad := range []string{launch + "; other", strings.Replace(launch, "$srProgressBefore", "$other", 1)} {
			p, s := proof, state
			p.LaunchCommand, s.LaunchDigest = bad, digest([]byte(bad))
			if usageInitialization(s, p) != nil {
				t.Fatal("modified progress wrapper accepted")
			}
		}
		if progressRun != "" {
			proof.RunID = strings.Repeat("b", 32)
			if usageInitialization(state, proof) != nil {
				t.Fatal("foreign run initialized")
			}
		}
		proof.RunID = run
		state.LaunchDigest = "different"
		if usageInitialization(state, proof) != nil {
			t.Fatal("full digest bypassed")
		}
	}
}
