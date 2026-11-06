package application

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestStopContractOnlineAndOfflineKeepScopeAndDecoderOrder(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	cli, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(contracts.OrcaSettings{AccessMode: "full_access", Transport: contracts.TransportSettings{Executable: cli}, AgentLaunch: contracts.AgentSettings{Model: "requested-model", ReasoningEffort: "medium"}})}
	f.request.Workflow = contracts.Workflow{ID: workflow.SingleExecution, Version: 1, PromptOverrides: map[string]string{"execute": "Verify the original scope."}}
	f.request.Publication = contracts.Publication{}
	f.input.Input = canonical(f.request)
	original := f.freeze()
	for _, tc := range []struct {
		name, scope, diagnostic, offlineDiagnostic string
		badCodec                                   bool
	}{
		{"saved-scope", orca.AgentSessionStopScope, "", "", false},
		// The original structured profile permits an absent saved scope. A
		// marker classification must not silently introduce a stronger gate.
		{"original-no-saved-scope", "", "", "", false},
		{"mismatched-scope", "other-scope", "frozen Orca stop evidence scope is missing or inconsistent", "frozen stop evidence scope is missing or inconsistent", false},
		{"decoder-before-scope", "other-scope", "invalid structured Orca profile codec", "invalid structured Orca profile codec", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := original.Clone()
			r.StopEvidenceScope, r.StopStatus = tc.scope, "stopped"
			r.Allowed, r.ResumeRequired = false, false
			r.Status, r.Phase = "stopped", "stopped"
			if tc.badCodec {
				r.Profile[0].CodecVersion = 99
				rebuilt, err := b.NewImplementationDataEnvelope(r.Profile[0], r.Profile[0].Payload)
				if err != nil {
					t.Fatal(err)
				}
				r.Profile[0] = rebuilt
			}
			before := r.Clone()
			e := &Engine{logger: log.New(io.Discard, "", 0)}
			w := &worker{}
			port, err := e.port(r, w)
			if tc.diagnostic == "" {
				if err != nil || port == nil || w.port != port {
					t.Fatal("original online scope admission changed", port, err)
				}
			} else if err == nil || err.Error() != tc.diagnostic || port != nil || w.port != nil {
				t.Fatal("online error priority or failed-port cache changed", port, err)
			}
			verified := VerifyStopped(context.Background(), f.state, []*runstate.Record{r})
			if verified.ResourcesSettled != (tc.diagnostic == "") || len(verified.Runs) != 1 {
				t.Fatal("offline scope admission changed", verified)
			}
			if tc.offlineDiagnostic != "" && !strings.Contains(verified.Runs[0].Diagnostic, tc.offlineDiagnostic) {
				t.Fatal("offline decoder/scope error was hidden", verified)
			}
			if !reflect.DeepEqual(before, r) {
				t.Fatal("scope admission or verification changed original record")
			}
		})
	}
}

func TestStopContractKeepsWorkflowAndSavedScopeRules(t *testing.T) {
	for _, tc := range []struct {
		workflow string
		version  int
		want     bool
	}{
		{"development", 2, true}, {"development-audited", 2, true},
		{"development", 1, false}, {"execute", 1, false}, {"conversation", 1, false},
	} {
		r := &runstate.Record{Request: contracts.RunRequest{
			Workflow:  contracts.Workflow{ID: tc.workflow, Version: tc.version},
			Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"full_access"}`)},
		}, Profile: []b.ImplementationDataEnvelope{{ImplementationKey: "orca.agent-session-profile", CodecVersion: 2, Payload: json.RawMessage(`{}`)}}}
		r.Snapshot.Ref.Protocol = b.ProtocolOrcaAgentSessionV1
		if got := structuredDevelopmentClose(r); got != tc.want {
			t.Fatal(tc, got)
		}
		for _, scope := range []string{"", "orca_terminal", "orca_agent_session", "orca_local_codex_resources", "other"} {
			for _, saved := range []string{"", "orca_agent_session", "other"} {
				r.StopEvidenceScope = saved
				if got := requiresFrozenOrcaScope(r, scope); got != (saved != "" || scope == "orca_local_codex_resources") {
					t.Fatal("saved scope rule changed", scope, saved, got)
				}
			}
		}
	}
}

func TestStructuredDevelopmentStopBeforeResourceKeepsAdmissionOrder(t *testing.T) {
	e, _ := snapshotExecutionFixture(t)
	r := e.runs["run"].Clone()
	r.Request.Workflow = contracts.Workflow{ID: "development", Version: 2}
	r.ResultFileContract = ""
	r.Request.ResultFile, r.ResultFile = nil, nil
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: "orca.agent-session-profile", CodecVersion: 2, Purpose: b.ImplementationConfigPurpose, ProjectID: r.Snapshot.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: "fixture-config"}, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Profile = []b.ImplementationDataEnvelope{profile}
	r.Snapshot.Ref.External = b.ProcessIdentity{}
	r.Snapshot.Operations, r.Intents = nil, nil
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	if response := contentControl(e, "stop", "before-resource-stop", contracts.RevisionInput{ExpectedRevision: r.Revision}); !response.OK {
		t.Fatal(response.Error)
	}
	w := e.workers["run"]
	e.stopRun("run", w, &confirmedStructuredDevelopmentStop{})
	stopped := e.runs["run"]
	if stopped.StopStatus != "stopped" || stopped.Allowed || stopped.ResumeRequired || len(stopped.Intents) != 0 {
		t.Fatal("pre-resource stop created an intent or retained authority", stopped.View())
	}
	for _, command := range []string{"stop", "resume"} {
		response := contentControl(e, command, "stale-after-stop-"+command, contracts.RevisionInput{ExpectedRevision: stopped.Revision - 1})
		if response.OK || response.Error.Code != "invalid_state" || e.runs["run"].Revision != stopped.Revision {
			t.Fatal("closed development admission moved after CAS or committed a rejection", response)
		}
	}
}
