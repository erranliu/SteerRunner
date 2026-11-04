package application

import (
	"bytes"
	"context"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os/exec"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func TestSRFreezeOrcaAgent(t *testing.T) {
	for _, agent := range []string{"", "codex", "pi"} {
		t.Run("agent="+agent, func(t *testing.T) {
			f := newBaseRefreshFixture(t, "origin", "main")
			executable, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			settings := contracts.OrcaSettings{AccessMode: "full_access", Agent: agent}
			settings.Transport.Executable = executable
			f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
			f.input.Input = canonical(f.request)
			r := f.freeze()
			want := settings.AgentName()
			if r.Snapshot.Ref.Agent.Name != want || r.Snapshot.Session.Agent.Name != want || r.View().Execution.Agent != want {
				t.Fatal("frozen agent or public projection was lost")
			}
			if r.LaunchCommand != "" {
				t.Fatal("default launch should not inject a terminal command", r.LaunchCommand)
			}
			if want == "codex" && (r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || r.StopEvidenceScope != orca.AgentSessionStopScope || r.Profile[0].CodecVersion != orca.AgentSessionCloseProfileVersion || resultContract(r) != contracts.ResultFileContract) {
				t.Fatal("default Codex development did not freeze the structured owner and contracts")
			}
			if want == "pi" && (r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || r.StopEvidenceScope != orca.AgentSessionStopScope || r.Profile[0].CodecVersion != orca.AgentSessionCloseProfileVersion || resultContract(r) != contracts.ResultFileSnapshotContract) {
				t.Fatal("local Pi did not freeze the native owner and snapshot contract")
			}
			if r.InputDigest != reliable.Digest(canonical(r.Request)) {
				t.Fatal("frozen request digest is inconsistent")
			}
			if agent == "" {
				for _, test := range []struct{ contract, instruction string }{
					{"", workflow.ResultFileInstruction},
					{contracts.ResultFileSnapshotContract, workflow.DevelopmentResultFileSnapshotInstruction},
				} {
					next := r.Clone()
					next.Request.ResultFile = &contracts.ResultFileInput{}
					next.ResultFileContract = test.contract
					if err := queueInput(next, workflow.StageInput{StepID: next.Workflow.StepID}, ""); err != nil {
						t.Fatal(err)
					}
					prompt := next.Snapshot.PendingTurn.Prompt
					if !strings.Contains(prompt, test.instruction) || strings.Contains(prompt, workflow.ResultFileSnapshotInstruction) {
						t.Fatal("incorrect development result file window", test.contract)
					}
				}
				settings.Transport.WaitTimeout = "30s"
				if !bytes.Equal(r.Request.Execution.Settings, canonical(settings)) {
					t.Fatal("default settings gained an explicit launch override", string(r.Request.Execution.Settings))
				}
			}
			if want == "codex" || want == "pi" {
				for _, override := range []contracts.AgentSettings{{Executable: "codex"}, {Args: []string{"--no-alt-screen"}}, {RuntimeExecutable: "missing-helper.exe"}} {
					settings.AgentLaunch = override
					f.request.Execution.Settings = canonical(settings)
					f.input.Input = canonical(f.request)
					if _, err := freeze(context.Background(), f.state, f.input); err == nil || !strings.Contains(err.Error(), "native executor") {
						t.Fatal("explicit launch override did not fail before startup", override, err)
					}
				}
			}
		})
	}
	t.Run("remote-explicit-launch-rejected", func(t *testing.T) {
		f := newBaseRefreshFixture(t, "origin", "main")
		exe, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		settings := contracts.OrcaSettings{AccessMode: "full_access"}
		settings.Transport.Executable = exe
		settings.AgentLaunch.Executable = "codex"
		settings.AgentLaunch.Model = "gpt-6-luna"
		settings.AgentLaunch.ReasoningEffort = "high"
		f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "original-remote"}, Settings: canonical(settings)}
		f.input.Input = canonical(f.request)
		if _, err := freeze(context.Background(), f.state, f.input); err == nil || !strings.Contains(err.Error(), "native executor") {
			t.Fatal("remote executor override reached the pairing or old route", err)
		}

	})
}
