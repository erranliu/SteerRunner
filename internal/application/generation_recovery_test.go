package application

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestServiceGenerationPreservesDurableAuthorizationAndBusinessState(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before := map[string]*runstate.Record{}
	for _, state := range []string{"queued", "running", "waiting_input", "blocked", "stopped", "pr_ready"} {
		r := &runstate.Record{ID: state, Revision: 1, Status: state, Phase: "original-phase", Allowed: state == "queued" || state == "running" || state == "waiting_input", ResumeRequired: state == "blocked" || state == "stopped", CommandStates: map[string]string{"original-resume": "pending"}, PendingResume: "original-resume", Diagnostic: "original-business-reason"}
		if err = s.Commit(r, 0, nil); err != nil {
			t.Fatal(err)
		}
		before[state] = r.Clone()
	}
	e, err := New(context.Background(), dir, s, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer e.cancel()
	for id, r := range before {
		if !reflect.DeepEqual(r, e.runs[id]) {
			t.Errorf("restart rewrote %s: before=%+v after=%+v", id, r, e.runs[id])
		}
	}
	if err = e.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A second generation must not manufacture revisions, revoke authorization
	// or fail the original pending resume receipt either.
	second, err := New(context.Background(), dir, s, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer second.cancel()
	for id, r := range before {
		if !reflect.DeepEqual(r, second.runs[id]) {
			t.Errorf("second restart rewrote %s", id)
		}
	}
}

func TestServiceGenerationUnknownOperationVersionNeverReachesNativeReconciliation(t *testing.T) {
	e, w, _ := operationEngine(t)
	p := &partialReceiptPort{}
	w.port = p
	e.advance("run", w)
	r := e.runs["run"].Clone()
	r.Intents[0].Request.Operation.Version = 99
	e.runs["run"] = r
	e.advance("run", w)
	revision := e.runs["run"].Revision
	e.advance("run", w)
	if p.executes != 1 || p.reconciles != 0 || e.runs["run"].Intents[0].Request.Operation.Status != b.OperationUnknown || e.runs["run"].Revision != revision {
		t.Fatal("unsupported operation reached native I/O, lost its unknown fact or manufactured repeated revisions")
	}
}

func TestServiceGenerationTakeoverRejectsUnknownCodecAndKeepsOriginalFacts(t *testing.T) {
	p, _ := workflow.Lookup("development", 2)
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: codex.ProfileCodecKey, CodecVersion: codex.ProfileCodecVersion, Purpose: b.ImplementationConfigPurpose, ProjectID: "project", ProfileID: "run", AcceptedConfigVersion: "input"}, []byte(`{"access_mode":"full_access","executable":"codex.exe"}`))
	if err != nil {
		t.Fatal(err)
	}
	r := &runstate.Record{ID: "run", InputDigest: "input", Profile: []b.ImplementationDataEnvelope{profile}, Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: p.ID, Version: p.Version}, Execution: contracts.Execution{Adapter: "codex-app-server", Settings: []byte(`{"access_mode":"full_access"}`)}}, Workflow: workflow.Snapshot{Config: &p}}
	r.Snapshot.Session.ProjectID = "project"
	r.Snapshot.Ref.Version = 1
	r.AcquisitionConfig.Config = ra.DefaultConfig()
	before := canonical(r)
	items := TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), workflow.Catalog())
	if len(items) != 1 || items[0].Status != "compatible" || !bytes.Equal(before, canonical(r)) {
		t.Fatalf("same contract report rejected or mutated original record: %+v", items)
	}
	withResult := r.Clone()
	withResult.Request.ResultFile = &contracts.ResultFileInput{}
	for _, contract := range []string{"", "steerrunner-result-file/v99", contracts.ResultFileContract} {
		item := TakeoverFacts([]*runstate.Record{withResult}, resources.RecoveryCapabilities(), workflow.Catalog(), contract)[0]
		if (item.Status == "compatible") != (contract == contracts.ResultFileContract) {
			t.Fatal("candidate file capability not fenced", contract, item)
		}
	}
	for _, version := range []int{0, 99} {
		copy := r.Clone()
		copy.Snapshot.Ref.Version = version
		items = TakeoverFacts([]*runstate.Record{copy}, resources.RecoveryCapabilities(), workflow.Catalog())
		if items[0].Status != "unsupported" {
			t.Fatalf("unknown execution record %d accepted", version)
		}
	}
	copy := r.Clone()
	copy.AcquisitionConfig = ra.FrozenConfig{}
	if item := TakeoverFacts([]*runstate.Record{copy}, resources.RecoveryCapabilities(), workflow.Catalog())[0]; item.Status != "compatible" {
		t.Fatal("unused parser configuration blocked takeover")
	}
	copy = r.Clone()
	copy.AcquisitionConfig.Config.Schema = "steerrunner-result-acquisition/v99"
	if item := TakeoverFacts([]*runstate.Record{copy}, resources.RecoveryCapabilities(), workflow.Catalog())[0]; item.Status != "compatible" {
		t.Fatal("unused parser configuration blocked takeover")
	}
	r.Profile[0].CodecVersion = 99
	items = TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), workflow.Catalog())
	if items[0].Status != "unsupported" || items[0].InputDigest != "input" {
		t.Fatal("unknown codec accepted or original input identity lost")
	}
}

func TestServiceGenerationControlScopeRetainsOnlyOriginalQuestionAndSource(t *testing.T) {
	r := &runstate.Record{ID: "run", Question: &contracts.QuestionView{ID: "question", Text: "original question"}}
	r.Snapshot.Session.AttemptID = "attempt"
	r.Snapshot.Session.Generation = "generation"
	r.Snapshot.Session.CurrentTurnID = "turn"
	r.Workflow.ImplementationResultID = "implementation"
	original := controlScope(r, "question")
	r.Revision++
	if controlScope(r, "question") != original {
		t.Fatal("business revision invalidated original scope")
	}
	for _, field := range []string{"attempt", "generation", "question", "source"} {
		copy := r.Clone()
		switch field {
		case "attempt":
			copy.Snapshot.Session.AttemptID = "replacement"
		case "generation":
			copy.Snapshot.Session.Generation = "replacement"
		case "question":
			copy.Question.Text = "another question"
		case "source":
			copy.Workflow.ImplementationResultID = "replacement"
		}
		if controlScope(copy, "question") == original {
			t.Fatalf("%s escaped frozen input scope", field)
		}
	}
}

func TestWorkflowRecoveryChecksExecutionSemanticsInsteadOfCatalogMetadata(t *testing.T) {
	current, _ := workflow.Lookup("development", 2)
	raw, _ := json.Marshal(current)
	var frozen workflow.Profile
	json.Unmarshal(raw, &frozen)
	frozen.Description = "original description"
	frozen.SupportedAdapters = append(frozen.SupportedAdapters, "old-unused-adapter")
	frozen.PromptSteps = []string{"original-admission-defaults"}
	state := frozen.States[frozen.Initial]
	state.Prompt = "original frozen default prompt"
	frozen.States[frozen.Initial] = state
	if !compatibleWorkflow(current, frozen, "codex-app-server") {
		t.Fatal("catalog metadata prevented original execution takeover")
	}
	frozen.Initial = "another-entry"
	if compatibleWorkflow(current, frozen, "codex-app-server") {
		t.Fatal("changed execution semantics accepted")
	}
}
