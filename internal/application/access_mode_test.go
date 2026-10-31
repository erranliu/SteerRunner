package application

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestRetiredDevelopmentNeverStartsBackgroundWorkOrRewritesAuthorization(t *testing.T) {
	for _, mode := range []string{"", "sandbox"} {
		t.Run(mode, func(t *testing.T) {
			e, w, _ := operationEngine(t)
			if !e.CanCollectUsage("run") {
				t.Fatal("supported full_access Run lost usage collection")
			}
			r := e.runs["run"].Clone()
			r.Request.Execution.Settings = canonical(map[string]any{"access_mode": mode})
			r.PendingPRURL = "https://invalid.example/pull/1"
			r.Request.ProgressFile = &contracts.ProgressFileInput{}
			before := r.Clone()
			e.runs["run"] = r
			if e.CanCollectUsage("run") || e.CanCollectUsage("missing") {
				t.Fatal("retired or missing Run authorized background log access")
			}
			e.launch("run", w)
			if w.started || w.progressWatching || e.advance("run", w) {
				t.Fatal("retired background work started")
			}
			if err := e.prepareProgress(r); err != nil {
				t.Fatal(err)
			}
			if err := e.prepareResult(r); err != nil {
				t.Fatal(err)
			}
			e.collectProgress(context.Background(), "run", "closed")
			e.sealResult(context.Background(), "run")
			if !reflect.DeepEqual(before, e.runs["run"]) {
				t.Fatal("retirement changed frozen facts or authorization")
			}
			visible := publicRunView(r)
			if visible.Controls.CanResume || visible.Controls.CanReply || visible.Controls.CanMessage || visible.Controls.CanInterrupt || visible.Controls.CanClose || !visible.Controls.CanStop {
				t.Fatal("retired controls expose new input", visible.Controls)
			}
			projected := readOnlyProjection(r)
			if projected.Allowed || !projected.ResumeRequired || projected.Request.Execution.Settings == nil {
				t.Fatal("unsafe read-only projection")
			}
			if !reflect.DeepEqual(before, r) {
				t.Fatal("projection mutated source")
			}
			takeover := TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), nil)
			if len(takeover) != 1 || takeover[0].Status != "read_only" {
				t.Fatal(takeover)
			}
			if _, err := e.port(r, w); err == nil {
				t.Fatal("retired normal lifecycle constructed")
			}
		})
	}
}

func TestFrozenAccessModeConflictsCannotGrantExecution(t *testing.T) {
	r := &runstate.Record{Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "development", Version: 2}, Execution: contracts.Execution{Settings: json.RawMessage(`{"access_mode":"full_access"}`)}}}
	if retiredDevelopment(r) {
		t.Fatal("explicit full_access retired")
	}
	r.Profile = []b.ImplementationDataEnvelope{{ImplementationKey: "codex.worker-profile", CodecVersion: codex.ProfileCodecVersion, Payload: json.RawMessage(`{"access_mode":"sandbox"}`)}}
	if !retiredDevelopment(r) {
		t.Fatal("conflicting profile granted execution")
	}
	r.Profile[0].Payload = json.RawMessage(`{"access_mode":"full_access"}`)
	if retiredDevelopment(r) {
		t.Fatal("supported explicit full_access profile retired")
	}
	r.Profile[0].CodecVersion = 999
	if !retiredDevelopment(r) || readOnlyStatus(r) != "unsupported" {
		t.Fatal("unknown frozen profile granted execution")
	}
	before := r.Profile[0]
	_ = publicRunView(r) // Corrupt envelopes remain inspectable without marshal.
	if !reflect.DeepEqual(before, r.Profile[0]) {
		t.Fatal("projection rewrote unsupported frozen profile")
	}
	r.Request.Workflow = contracts.Workflow{ID: "generate", Version: 1}
	r.Request.Execution.Settings = json.RawMessage(`{"access_mode":"read_only"}`)
	if retiredDevelopment(r) {
		t.Fatal("content read_only retired")
	}
}
