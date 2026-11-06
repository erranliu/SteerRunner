package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestStructuredConstructorFacts(t *testing.T) {
	for _, tc := range []struct {
		name, agent, environment string
		version                  int
		development, audited     bool
	}{
		{"local-execute", "codex", "", 1, false, false},
		{"local-development-v1", "codex", "", 1, true, false},
		{"local-development-v2", "codex", "", 2, true, false},
		{"local-audited", "codex", "", 2, true, true},
		{"local-pi", "pi", "", 2, true, false},
		{"paired-execute-v1", "codex", "paired", 1, false, false},
		{"paired-execute-v2", "codex", "paired", 2, false, false},
		{"paired-development", "codex", "paired", 2, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _, _ := structuredFixture(t)
			setup := l.setup.Clone()
			host := setup.ProcessHost.(*fakeHost)
			p := AgentSessionProfile{Transport: Profile{Executable: "orca.exe", WaitTimeout: "1s", Environment: tc.environment}, AccessMode: "full_access", Model: "requested-model", ReasoningEffort: "medium", Development: tc.development, Audited: tc.audited}
			if tc.environment != "" {
				p.PairingHostID = "sha256:" + strings.Repeat("ab", 32)
			}
			setup.Snapshot.Ref.Agent.Name = tc.agent
			setup.Snapshot.Ref.Target = p.Transport.Target()
			setup.ProfileData[0].CodecVersion = tc.version
			setup.ProfileData[0].Payload = mustAgentJSON(p)
			var err error
			setup.ProfileData[0], err = b.NewImplementationDataEnvelope(setup.ProfileData[0], setup.ProfileData[0].Payload)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(setup)
			r, err := decodeFrozenRecord(setup)
			if err != nil {
				t.Fatal(err)
			}
			port, err := NewLifecycle(setup)
			if err != nil {
				t.Fatal(err)
			}
			execution := port.(*Lifecycle)
			for _, facts := range []*frozenRecord{r, execution.frozenRecord()} {
				if !reflect.DeepEqual(facts.profile, p.Transport) || facts.profileVersion != tc.version || facts.agentHostID != p.PairingHostID || facts.agentDevelopment != tc.development || facts.agentAudited != tc.audited || facts.probeProcess == nil || facts.setup.ProcessHost != setup.ProcessHost {
					t.Fatalf("derived facts differ: %+v", facts)
				}
			}
			if execution.clock == nil || !execution.nativeAccess || execution.agentAudited != tc.audited {
				t.Fatal("execution initialization lost")
			}
			reader, err := newFrozenReader(setup)
			if err != nil {
				t.Fatal(err)
			}
			if _, executable := any(reader).(b.LifecyclePort); executable {
				t.Fatal("frozen reader gained execution")
			}
			if _, _, err := reader.client.CloseTerminal(context.Background(), "original", "runtime"); err == nil || !strings.Contains(err.Error(), "does not permit mutation") {
				t.Fatal("structured reader allowed mutation", err)
			}
			after, _ := json.Marshal(setup)
			if string(before) != string(after) || len(host.calls) != 0 {
				t.Fatal("constructor mutated input or performed I/O")
			}
			setup.ProfileData[0].Payload[0] = '!'
			if r.setup.ProfileData[0].Payload[0] != '{' || execution.setup.ProfileData[0].Payload[0] != '{' {
				t.Fatal("configuration clone aliases source")
			}
			if _, err := NewLifecycle(setup); err == nil {
				t.Fatal("new construction reused earlier validation")
			}
		})
	}
}

func TestStructuredConstructorFaultPriority(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*b.LifecycleSetup)
	}{
		{"missing", func(s *b.LifecycleSetup) { s.ProfileData = nil }},
		{"duplicate", func(s *b.LifecycleSetup) { s.ProfileData = append(s.ProfileData, s.ProfileData[0]) }},
		{"codec-before-json", func(s *b.LifecycleSetup) { s.ProfileData[0].CodecVersion = -1; s.ProfileData[0].Payload = []byte(`!`) }},
		{"json-before-identity", func(s *b.LifecycleSetup) {
			s.ProfileData[0].ProjectID = "another-project"
			s.ProfileData[0].Payload = []byte(`!`)
		}},
		{"identity", func(s *b.LifecycleSetup) { s.ProfileData[0].ProjectID = "another-project" }},
		{"json", func(s *b.LifecycleSetup) { s.ProfileData[0].Payload = []byte(`!`) }},
		{"unknown-field", func(s *b.LifecycleSetup) {
			s.ProfileData[0].Payload = append(s.ProfileData[0].Payload[:len(s.ProfileData[0].Payload)-1], []byte(`,"extra":true}`)...)
		}},
		{"trailing-json", func(s *b.LifecycleSetup) {
			s.ProfileData[0].Payload = append(s.ProfileData[0].Payload, []byte(` {}`)...)
		}},
		{"target", func(s *b.LifecycleSetup) { s.Snapshot.Ref.Target.ID = "another-target" }},
		{"retired-launch", func(s *b.LifecycleSetup) { s.AgentLaunchCommand = "retired" }},
		{"pi-v1", func(s *b.LifecycleSetup) { s.Snapshot.Ref.Agent.Name = "pi" }},
		{"local-host", func(s *b.LifecycleSetup) {
			var p AgentSessionProfile
			_ = json.Unmarshal(s.ProfileData[0].Payload, &p)
			p.PairingHostID = "sha256:" + strings.Repeat("ab", 32)
			s.ProfileData[0].Payload = mustAgentJSON(p)
		}},
		{"invalid-audited", func(s *b.LifecycleSetup) {
			var p AgentSessionProfile
			_ = json.Unmarshal(s.ProfileData[0].Payload, &p)
			p.Audited = true
			s.ProfileData[0].Payload = mustAgentJSON(p)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _, _ := structuredFixture(t)
			setup := l.setup.Clone()
			tc.change(&setup)
			for i, e := range setup.ProfileData {
				if e.CodecVersion > 0 && json.Valid(e.Payload) {
					var err error
					setup.ProfileData[i], err = b.NewImplementationDataEnvelope(e, e.Payload)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			_, want := decodeAgentSessionProfile(setup)
			if want == nil {
				t.Fatal("invalid fixture succeeded")
			}
			r, got := decodeFrozenRecord(setup)
			assertConstructorError(t, got, want)
			port, got := NewLifecycle(setup)
			assertConstructorError(t, got, want)
			if r != nil || port != nil {
				t.Fatal("invalid profile returned usable owner")
			}
			setup.ProcessHost = nil
			_, got = decodeFrozenRecord(setup)
			assertConstructorError(t, got, errors.New("unsupported Orca terminal lifecycle"))
			_, got = NewLifecycle(setup)
			assertConstructorError(t, got, errors.New("unsupported Orca terminal lifecycle"))
			setup.Snapshot.Ref.Agent.Name = "retired-agent"
			_, got = decodeFrozenRecord(setup)
			assertConstructorError(t, got, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Orca agent execution/history is unsupported; only Codex and Pi are supported."})
			_, got = NewLifecycle(setup)
			assertConstructorError(t, got, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Orca agent execution/history is unsupported; only Codex and Pi are supported."})
			setup.Snapshot.Ref.Protocol = b.ProtocolOrcaTerminalV3
			_, got = NewLifecycle(setup)
			assertConstructorError(t, got, &b.BackendError{Code: b.BackendUnsupported, Diagnostic: "Legacy Orca terminal execution must be drained with its original Runner build."})
		})
	}
}

func assertConstructorError(t *testing.T, got, want error) {
	t.Helper()
	if got == nil || reflect.TypeOf(got) != reflect.TypeOf(want) || got.Error() != want.Error() {
		t.Fatalf("error changed: got %T %v; want %T %v", got, got, want, want)
	}
	var actual, expected *b.BackendError
	if errors.As(want, &expected) && (!errors.As(got, &actual) || actual.Code != expected.Code) {
		t.Fatal("error classification changed")
	}
}
