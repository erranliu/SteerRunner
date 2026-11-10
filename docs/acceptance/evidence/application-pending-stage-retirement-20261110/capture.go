// One-shot acceptance capture, outside product/test entry points.
package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"time"
)

func main() {
	types := map[string]string{}
	for _, dir := range []string{"internal/application", "internal/execution/workflow", "internal/execution/model", "internal/execution/reliable", "internal/execution/backend", "internal/runstate"} {
		must(filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path, nil, 0)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				g, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range g.Specs {
					t, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					var buf bytes.Buffer
					if err := format.Node(&buf, set, t); err != nil {
						return err
					}
					types[filepath.ToSlash(path)+"::"+t.Name.Name] = buf.String()
				}
			}
			return nil
		}))
	}

	text, budget, err := m.FreezeInput(m.InputPart{Source: "workflow.body", Owner: m.CallerInput, Text: "original 中文\n"})
	must(err)
	input := workflow.StageInput{CommandID: "report", AttemptID: "attempt", StepID: workflow.StepReport, Version: 2, Prompt: text, Budget: budget, SourceCommandID: "implementation", SourceTurnID: "original-turn", SourceVersion: 1, SourceResultID: "source-result", ReportToolPolicy: "original-policy", Model: "original-model", ReasoningEffort: "medium"}
	snapshot := b.LifecycleSnapshot{Session: b.SessionContext{SessionID: "original-session", AttemptID: "attempt", PendingCommandID: input.CommandID}, PendingTurn: &b.TurnInput{CommandID: input.CommandID, StepID: input.StepID, InputVersion: input.Version, Prompt: input.Prompt, Budget: input.Budget.Clone(), ReportToolPolicy: input.ReportToolPolicy, Model: input.Model, ReasoningEffort: input.ReasoningEffort}}
	record := runstate.Record{ID: "run", Revision: 1, Allowed: false, ResumeRequired: true, Snapshot: snapshot, Workflow: workflow.Snapshot{StepID: input.StepID, StageInputs: []workflow.StageInput{input}}, CommandStates: map[string]string{"resume": "pending"}}
	records := map[string]runstate.Record{"frozen_pending": record}
	draft := record.Clone()
	draft.Snapshot.PendingTurn = nil
	draft.NextStage = &workflow.StageInput{StepID: workflow.StepAuditCode, SourceCommandID: "implementation", SourceTurnID: "original-turn", SourceVersion: 1, SourceResultID: "source-result"}
	records["draft_successor"] = *draft
	frozen := record.Clone()
	frozen.Snapshot.PendingTurn = nil
	frozen.NextStage = &input
	records["frozen_next_stage"] = *frozen
	receipt := b.LifecycleReceipt{Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{SessionID: "original-session", TurnID: "report-turn", InputAccepted: true, TurnStarted: true}}
	consumed := record.Clone()
	consumed.Snapshot, consumed.NativeStarted, err = reliable.ApplySessionReceipt(snapshot, nil, receipt, false)
	must(err)
	records["consumed"] = *consumed
	unknown := record.Clone()
	unknown.Intents = []operation.Intent{{Request: b.LifecycleRequest{Operation: b.Operation{ID: "original-operation", Status: b.OperationUnknown, Action: string(b.LifecycleStartTurn), StartedAt: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}, Snapshot: snapshot, Input: b.LifecycleInput{Turn: snapshot.PendingTurn}}}}
	records["unknown"] = *unknown
	stopped := record.Clone()
	stopped.PendingStop = "stop"
	stopped.StopStatus = "pending"
	records["stop"] = *stopped
	raw := marshal(records)
	var decoded map[string]runstate.Record
	must(json.Unmarshal(raw, &decoded))
	if !bytes.Equal(raw, marshal(decoded)) {
		panic("record JSON roundtrip changed bytes")
	}
	values := map[string]any{"types": types, "records": records, "receipt": receipt, "capabilities": resources.Capabilities(), "schemas": json.RawMessage(resources.ConfigurationSchemas()), "resource_digest": resources.Digest()}
	must(os.WriteFile(os.Args[1], marshal(values), 0600))
}
func marshal(v any) []byte { raw, err := json.Marshal(v); must(err); return raw }
func must(err error) {
	if err != nil {
		panic(err)
	}
}
