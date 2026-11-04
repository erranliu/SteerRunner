package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestSRFreezeOrcaStructuredExecuteOnly(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	cli, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	settings := contracts.OrcaSettings{AccessMode: "full_access", Transport: contracts.TransportSettings{Executable: cli}, AgentLaunch: contracts.AgentSettings{Model: "requested-model", ReasoningEffort: "high"}}
	f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
	f.request.Workflow = contracts.Workflow{ID: workflow.SingleExecution, Version: 1, PromptOverrides: map[string]string{"execute": "one\r\n\r\n中文😀\n```\n<&> \\path\n```  \r\n"}}
	f.request.Publication = contracts.Publication{}
	f.input.Input = canonical(f.request)
	r := f.freeze()
	if r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || r.LaunchCommand != "" || r.LaunchInput != nil || r.StopEvidenceScope != orca.AgentSessionStopScope || r.Profile[0].ImplementationKey != orca.AgentSessionProfileKey {
		t.Fatal("new execute did not freeze native ownership", r.LaunchCommand, r.Snapshot.Ref, r.Profile)
	}
	var profile orca.AgentSessionProfile
	if err = json.Unmarshal(r.Profile[0].Payload, &profile); err != nil || profile.Model != "requested-model" || profile.ReasoningEffort != "high" || profile.AccessMode != "full_access" || profile.Transport.Terminal != nil || profile.Transport.RuntimeExecutable != "" {
		t.Fatal(profile, err)
	}
	// Persistent factory reconstruction uses the frozen protocol, not a new
	// workflow/combination lookup. Neither construction starts an agent/helper.
	store, err := storage.Open(f.state)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Commit(r, 0, nil); err != nil {
		t.Fatal(err)
	}
	records, err := store.Runs()
	if err != nil || len(records) != 1 {
		t.Fatal(err, len(records))
	}
	engine := &Engine{dir: f.state}
	port, err := orca.NewLifecycle(engine.setup(records[0], &worker{}))
	if err != nil || port.PolicyCapabilities().Preparation.Resource != b.MechanismNotApplicable {
		t.Fatal("frozen factory rejected native protocol", err)
	}
	// Resource cleanup remains unknown while the committed business result is readable.
	r.ContentResult = &contracts.ContentResult{Status: "succeeded", Text: "complete result"}
	r.Status = "reconciling"
	r.StopStatus = "unknown"
	r.Closed = true
	if v := r.View(); v.ContentResult == nil || v.ContentResult.Text != "complete result" || v.StopStatus != "unknown" || v.Status != "reconciling" {
		t.Fatal(v)
	}
	// Every explicit old executor setting is rejected before helper lookup.
	for _, launch := range []string{`{"runtime_executable":"missing-helper.exe"}`, `{"runtime_executable":""}`, `{"runtime_sha256":""}`, `{"executable":"missing-codex.exe"}`, `{"args":["--old"]}`} {
		request := f.request
		request.Execution.Settings = json.RawMessage(`{"access_mode":"full_access","agent_launch":` + launch + `}`)
		input := f.input
		input.Input = canonical(request)
		_, err = freeze(context.Background(), f.state, input)
		if err == nil || !strings.Contains(err.Error(), "native executor") {
			t.Fatal("explicit executor was probed or silently ignored", launch, err)
		}
	}
	// Unknown/retired agents fail per record without corrupting the store.
	retired := r.Clone()
	retired.ID = "retired-dsh-record"
	retired.Snapshot.Ref.Agent.Name = "dsh"
	retired.Snapshot.Ref.Protocol = b.ProtocolOrcaTerminalV3
	retired.Request.Execution.Settings = canonical(contracts.OrcaSettings{Agent: "dsh", AccessMode: "full_access"})
	if err = store.Commit(retired, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = orca.NewLifecycle(engine.setup(retired, &worker{})); err == nil {
		t.Fatal("retired agent decoded as Codex")
	}
	if records, err = store.Runs(); err != nil || len(records) != 2 {
		t.Fatal("one unsupported record affected unrelated history", err)
	}
	for _, record := range records {
		_, err = orca.NewLifecycle(engine.setup(record, &worker{}))
		if (err == nil) != (record.ID == r.ID) {
			t.Fatal("record-level agent isolation changed", record.ID, err)
		}
	}
}

func TestStructuredFileEnvironmentBindings(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	executable, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(contracts.OrcaSettings{AccessMode: "full_access", Transport: contracts.TransportSettings{Executable: executable}})}
	f.request.Workflow = contracts.Workflow{ID: workflow.SingleExecution, Version: 1, PromptOverrides: map[string]string{"execute": "caller command must remain unchanged"}}
	f.request.Publication = contracts.Publication{}
	f.request.ResultFile = &contracts.ResultFileInput{}
	f.request.ProgressFile = &contracts.ProgressFileInput{}
	f.input.Input = canonical(f.request)
	r := f.freeze()
	if err := queueInput(r, workflow.StageInput{StepID: r.Workflow.StepID}, ""); err != nil {
		t.Fatal(err)
	}
	prompt := r.Snapshot.PendingTurn.Prompt
	for _, want := range []string{workflow.OrcaFileEnvironmentInstruction, workflow.ResultFileInstruction, workflow.ProgressInstruction, `"STEERLINE_RESULT_FILE":".steerrunner/result/` + r.ID + `/result.bin"`, `"STEERLINE_PROGRESS_FILE":".steerrunner/progress/` + r.ID + `/progress.bin"`, "caller command must remain unchanged"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing original file binding %q in %s", want, prompt)
		}
	}
}

func TestSRFreezeRemoteStructuredExecutePinsHostAndSealContract(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	cli, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	store := map[string]any{"environments": []any{map[string]any{"id": "original-remote", "preferredEndpointId": "endpoint", "endpoints": []any{map[string]string{"id": "endpoint", "endpoint": "ws://127.0.0.1:1", "deviceToken": "fixture-token", "publicKeyB64": base64.StdEncoding.EncodeToString(make([]byte, 32))}}}}}
	if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), canonical(store), 0600); err != nil {
		t.Fatal(err)
	}
	f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "id:original-remote"}, Settings: canonical(contracts.OrcaSettings{AccessMode: "full_access", Transport: contracts.TransportSettings{Executable: cli}, AgentLaunch: contracts.AgentSettings{Model: "requested-model", ReasoningEffort: "high"}})}
	f.request.Workflow = contracts.Workflow{ID: workflow.SingleExecution, Version: 1, PromptOverrides: map[string]string{"execute": "one\r\n\r\n中文😀  \n"}}
	f.request.Publication = contracts.Publication{}
	f.request.ResultFile = &contracts.ResultFileInput{InitialContentBase64: ""}
	f.input.Input = canonical(f.request)
	r := f.freeze()
	if err := queueInput(r, workflow.StageInput{StepID: r.Workflow.StepID}, ""); err != nil {
		t.Fatal(err)
	}
	prompt := r.Snapshot.PendingTurn.Prompt
	for _, want := range []string{workflow.ResultFileInstruction, workflow.OrcaFileEnvironmentInstruction, `"STEERLINE_RESULT_FILE":".steerrunner/result/` + r.ID + `/result.bin"`, f.request.Workflow.PromptOverrides["execute"]} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing remote file binding or original body %q in %s", want, prompt)
		}
	}
	if strings.Contains(prompt, workflow.ResultFileSnapshotInstruction) {
		t.Fatal("new remote seal was presented as a business-end snapshot")
	}
	var profile orca.AgentSessionProfile
	if err := json.Unmarshal(r.Profile[0].Payload, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.PairingHostID != "sha256:66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925" || profile.Transport.Environment != "id:original-remote" || profile.Transport.NativeTerminal || r.Profile[0].CodecVersion != orca.AgentSessionPairedCloseProfileVersion || resultContract(r) != contracts.ResultFileContract || r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || r.LaunchCommand != "" || r.LaunchInput != nil {
		t.Fatal("remote native owner/host/contract not frozen", profile, r.Snapshot.Ref, r.ResultFileContract)
	}
	engine := &Engine{dir: f.state, runs: map[string]*runstate.Record{r.ID: r}}
	if v := engine.HandleResultFile(r.ID).Data.(contracts.ResultFileView); v.Contract != contracts.ResultFileContract || v.CollectionStatus != "pending" {
		t.Fatal("pending query hid new contract", v)
	}
	port, err := orca.NewLifecycle(engine.setup(r, &worker{}))
	if err != nil || port.PolicyCapabilities().Preparation.Resource != b.MechanismNotApplicable {
		t.Fatal("frozen remote factory required a helper or connected on construction", err)
	}
	// Construction and admission use only the pairing metadata, never this
	// deliberately unavailable network endpoint. Old records keep their protocol.
	old := r.Clone()
	old.ResultFileContract = contracts.ResultFileSnapshotContract
	if resultContract(old) != contracts.ResultFileSnapshotContract {
		t.Fatal("frozen snapshot was reinterpreted")
	}
	old.ResultFileContract = ""
	old.Snapshot.Ref.Protocol = b.ProtocolOrcaTerminalV3
	if resultContract(old) != contracts.ResultFileContract {
		t.Fatal("old remote record reinterpreted")
	}
}
