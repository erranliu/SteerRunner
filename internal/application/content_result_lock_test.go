package application

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// Block the original verifier at the actual result boundary, without replacing
// any native port, persistence operation or shared material fixture.
func blockedContentFinish(t *testing.T, e *Engine, w *worker, value b.TurnEvidence) (<-chan bool, func()) {
	t.Helper()
	entered, released := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(released) }) }
	done := make(chan bool, 1)
	go func() {
		w.gate.Lock()
		defer w.gate.Unlock()
		done <- e.finishWithMaterials("run", value, &w.gate, func(r *runstate.Record) error {
			close(entered)
			<-released
			return verifyFrozenMaterials(r)
		})
	}()
	t.Cleanup(release)
	awaitEvidenceIO(t, entered)
	return done, release
}

func contentFinished(t *testing.T, done <-chan bool) bool {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("content result did not leave its commit boundary")
		return false
	}
}

func TestContentMaterialReadReleasesGlobalLockKeepsRunGate(t *testing.T) {
	e, w := contentEngine(t, "generate")
	value := contentEvidence(e, "original result", "succeeded", true)
	other := e.runs["run"].Clone()
	other.ID = "other"
	e.workers[other.ID] = &worker{wake: make(chan struct{}, 1)}
	if err := e.commitLocked(other, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	done, release := blockedContentFinish(t, e, w, value)
	if w.gate.TryLock() {
		w.gate.Unlock()
		t.Fatal("material read released the original Run dispatch gate")
	}
	responses := make(chan contracts.Response, 2)
	go func() {
		responses <- e.Handle("inspect", "other", "", "", "", 0, nil)
		raw := canonical(contracts.RevisionInput{ExpectedRevision: other.Revision})
		responses <- e.Handle("stop", "other", "", "", "", 0, &contracts.Request{Protocol: contracts.Protocol, RequestID: "other-stop", Input: raw})
	}()
	for i := 0; i < 2; i++ {
		if response := evidenceResponse(t, responses); !response.OK {
			t.Fatal(response)
		}
	}
	e.mu.Lock()
	uncommitted := e.runs["run"].ContentResult == nil && len(w.wake) == 0
	revoked := !e.runs["other"].Allowed && e.runs["other"].PendingStop != ""
	e.mu.Unlock()
	if !uncommitted || !revoked {
		t.Fatal("material sample published a result or blocked the other stop commit")
	}
	select {
	case <-done:
		t.Fatal("result confirmed before the material sample")
	default:
	}
	release()
	if !contentFinished(t, done) || e.runs["run"].ContentResult.Text != "original result" {
		t.Fatal("original result did not commit")
	}
}

func TestContentMaterialReadSerializesPublicStopAtOriginalCommit(t *testing.T) {
	for _, stale := range []bool{false, true} {
		name := "revision-after-final"
		if stale {
			name = "stale-revision-before-final"
		}
		t.Run(name, func(t *testing.T) {
			e, w := contentEngine(t, "conversation")
			value := contentEvidence(e, "original final", "succeeded", true)
			original := e.runs["run"].Clone()
			done, release := blockedContentFinish(t, e, w, value)
			revision := original.Revision + 1
			if stale {
				revision = original.Revision
			}
			started := make(chan struct{})
			stopped := make(chan contracts.Response, 1)
			go func() {
				close(started)
				stopped <- contentControl(e, "stop", "ordered-stop", contracts.RevisionInput{ExpectedRevision: revision})
			}()
			<-started
			if w.gate.TryLock() {
				w.gate.Unlock()
				t.Fatal("material sampling released the same Run gate")
			}
			select {
			case response := <-stopped:
				t.Fatal("public stop replied before the original result commit", response)
			default:
			}
			release()
			if !contentFinished(t, done) {
				t.Fatal("original final did not commit before stop")
			}
			response := evidenceResponse(t, stopped)
			if stale {
				if response.OK || response.Error.Code != "stale_revision" || e.runs["run"].PendingStop != "" {
					t.Fatal("stop checked revision before acquiring its original gate", response)
				}
				response = contentControl(e, "stop", "fresh-stop", contracts.RevisionInput{ExpectedRevision: original.Revision + 1})
			}
			if !response.OK {
				t.Fatal(response)
			}
			command := response.Data.(map[string]any)["command_id"].(string)
			r := e.runs["run"]
			if r.Revision != original.Revision+2 || r.Allowed || !r.Closed || r.PendingStop != command || r.ContentResult.Text != "original final" || len(r.History) != len(original.History)+1 || len(r.Workflow.StageInputs) != len(original.Workflow.StageInputs) || len(r.Intents) != 0 || r.Snapshot.PendingTurn != nil || r.NextStage != nil {
				t.Fatal("public stop changed the result/authority/dispatch boundary")
			}
			stored, err := e.store.Runs()
			if err != nil || len(stored) != 1 || string(canonical(stored[0])) != string(canonical(r)) {
				t.Fatal("public stop replied without its result and revocation committed", err)
			}
		})
	}
}

func TestContentMaterialSampleRejectsChangedRevisionAndReplaysOriginal(t *testing.T) {
	for _, change := range []string{"observation", "source", "stop"} {
		t.Run(change, func(t *testing.T) {
			e, w := contentEngine(t, "generate")
			value := contentEvidence(e, "original result", "succeeded", true)
			history := len(e.runs["run"].History)
			done, release := blockedContentFinish(t, e, w, value)
			e.mu.Lock()
			next := e.runs["run"].Clone()
			switch change {
			case "source":
				next.InputDigest = "new-source"
			case "stop":
				closeContent(next, "original-stop")
			}
			err := e.commitLocked(next, nil, "concurrent_fact")
			before := string(canonical(e.runs["run"]))
			e.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			release()
			if contentFinished(t, done) || string(canonical(e.runs["run"])) != before || len(w.wake) != 0 {
				t.Fatal("stale material sample crossed the result commit boundary")
			}
			if !e.finish("run", value) || e.finish("run", value) || len(e.runs["run"].History) != history+1 {
				t.Fatal("original result could not be acquired once from the current snapshot")
			}
			if change == "stop" {
				r := e.runs["run"]
				if r.Allowed || r.PendingStop != "original-stop" || !r.Closed || r.Snapshot.PendingTurn != nil || r.NextStage != nil || len(r.Workflow.StageInputs) != 1 || len(r.Intents) != 0 {
					t.Fatal("late result replaced stop or authorized new input")
				}
			}
		})
	}
}

func TestContentMaterialResultCommitFailureKeepsOriginalReplay(t *testing.T) {
	e, w := contentEngine(t, "generate")
	value := contentEvidence(e, "original result", "succeeded", true)
	if err := e.commitLocked(e.runs["run"].Clone(), nil, "observed"); err != nil {
		t.Fatal(err)
	}
	before := string(canonical(e.runs["run"]))
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	failOperationCommit(t, db)
	if e.finish("run", value) || string(canonical(e.runs["run"])) != before || len(w.wake) != 0 {
		t.Fatal("uncommitted content result was published or confirmed")
	}
	stored, err := e.store.Runs()
	if err != nil || len(stored) != 1 || string(canonical(stored[0])) != before {
		t.Fatal("failed result transaction changed the original stored record", err)
	}
	if _, err := db.Exec("DROP TRIGGER fail_operation_event"); err != nil {
		t.Fatal(err)
	}
	if !e.finish("run", value) || e.finish("run", value) || len(e.runs["run"].Workflow.StageResults) != 1 {
		t.Fatal("original result was lost or duplicated after commit recovery")
	}
}

func TestContentMaterialResultErrorPriority(t *testing.T) {
	for _, tc := range []struct {
		name, text, outcome, want string
		verified, sample          bool
	}{
		{"material-before-schema", "not-json", "succeeded", "repository_snapshot_changed", true, true},
		{"oversized", strings.Repeat("x", contracts.MaxGenerateResultBytes+1), "succeeded", "generate_result_limit_exceeded_or_invalid_utf8", true, false},
		{"invalid-utf8", string([]byte{0xff}), "succeeded", "generate_result_limit_exceeded_or_invalid_utf8", true, false},
		{"failed", "not-json", "failed", "", true, false},
		{"cancelled", "not-json", "cancelled", "", true, false},
		{"missing-final", "not-json", "succeeded", "complete_final_unavailable", false, false},
		{"wrong-source", "not-json", "succeeded", "final_source_unverified", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, w := contentEngine(t, "generate")
			e.runs["run"].Request.Generate.OutputSchema = []byte(`{"type":"object"}`)
			value := contentEvidence(e, tc.text, tc.outcome, tc.verified)
			if tc.name == "wrong-source" {
				value.FinalSessionID = "different-route"
			}
			called := false
			w.gate.Lock()
			accepted := e.finishWithMaterials("run", value, &w.gate, func(*runstate.Record) error {
				called = true
				return errors.New("material drift")
			})
			w.gate.Unlock()
			r := e.runs["run"]
			if called != tc.sample {
				t.Fatal("changed material sampling eligibility")
			}
			if tc.name == "wrong-source" {
				if accepted || r.Phase != tc.want || r.ContentResult != nil {
					t.Fatal("source failure lost its original priority")
				}
			} else if !accepted || r.ContentResult == nil || r.ContentResult.Diagnostic != tc.want {
				t.Fatalf("wrong result priority: accepted=%v result=%+v", accepted, r.ContentResult)
			}
		})
	}
}
