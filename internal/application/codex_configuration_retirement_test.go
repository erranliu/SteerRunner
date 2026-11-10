package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestCodexAdmissionPreservesFrozenDefaults(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	defaultExecutable := filepath.Join(bin, "codex.exe")
	if err := os.WriteFile(defaultExecutable, image, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, flow := range []string{"development", "generate", "conversation"} {
		t.Run(flow, func(t *testing.T) {
			request := f.request
			mode := "full_access"
			stateDir := f.state
			if flow != "development" {
				mode = "read_only"
				request = contracts.RunRequest{Title: "Frozen Codex defaults", Workflow: contracts.Workflow{ID: flow, Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}}}
				if flow == "generate" {
					request.Generate = &contracts.GenerateInput{Instruction: "Reply from the fixed input", Materials: []contracts.Document{}}
				} else {
					request.Conversation = &contracts.ConversationInput{Instruction: "Discuss the fixed input", Materials: []contracts.Document{}, Directory: contracts.DirectoryAccess{Path: t.TempDir(), Access: "read_only"}}
				}
				stateDir = t.TempDir()
			}
			marshal := func(t *testing.T, settings map[string]any) contracts.Request {
				t.Helper()
				var err error
				request.Execution.Settings, err = json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				return contracts.Request{Protocol: contracts.Protocol, RequestID: contracts.NewID(), Input: raw}
			}
			gate := func(t *testing.T, input contracts.Request) *contracts.Fault {
				t.Helper()
				var run contracts.RunRequest
				if err := json.Unmarshal(input.Input, &run); err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(contracts.RunConfiguration{Execution: run.Execution, Workflow: run.Workflow})
				if err != nil {
					t.Fatal(err)
				}
				return contracts.ValidateRunConfiguration(raw)
			}
			for _, explicitExecutable := range []bool{false, true} {
				for _, effort := range []string{"", "high"} {
					for _, model := range []string{"", "chosen-model"} {
						t.Run(fmt.Sprintf("explicit-exe-%t/effort-%s/model-%s", explicitExecutable, effort, model), func(t *testing.T) {
							settings := map[string]any{"access_mode": mode}
							wantExecutable, wantEffort := defaultExecutable, "medium"
							if explicitExecutable {
								settings["executable"], wantExecutable = executable, executable
							}
							if effort != "" {
								settings["reasoning_effort"], wantEffort = effort, effort
							}
							if model != "" {
								settings["model"] = model
							}
							input := marshal(t, settings)
							before := bytes.Clone(input.Input)
							if fault := gate(t, input); fault != nil {
								t.Fatal("invalid positive fixture", fault)
							}
							r, err := freeze(context.Background(), stateDir, input)
							if err != nil {
								t.Fatal(err)
							}
							wantExecutable, err = filepath.EvalSymlinks(wantExecutable)
							if err != nil {
								t.Fatal(err)
							}
							var frozen contracts.CodexSettings
							if err := json.Unmarshal(r.Request.Execution.Settings, &frozen); err != nil {
								t.Fatal(err)
							}
							if frozen != (contracts.CodexSettings{AccessMode: mode, Executable: wantExecutable, Model: model, ReasoningEffort: wantEffort}) {
								t.Fatal("effective request changed fixed defaults or explicit choices", frozen)
							}
							if len(r.Profile) != 1 {
								t.Fatal("missing frozen profile", len(r.Profile))
							}
							profile, err := codex.DecodeProfileConfig(r.Profile[0].Payload)
							if err != nil {
								t.Fatal(err)
							}
							if profile != (codex.ProfileConfig{AccessMode: mode, Executable: wantExecutable, Model: model, ReasoningEffort: wantEffort}) || r.Profile[0].CodecVersion != codex.ProfileVersionForMode(mode) {
								t.Fatal("frozen profile differs from accepted choices", profile)
							}
							if r.Workflow.ExecutionModel == nil || r.Workflow.ExecutionModel.Model != model || r.Workflow.ExecutionModel.ReasoningEffort != wantEffort || r.InputDigest != reliable.Digest(canonical(r.Request)) {
								t.Fatal("workflow choices or effective request digest lost", r.Workflow.ExecutionModel)
							}
							if !bytes.Equal(before, input.Input) || len(r.Intents) != 0 {
								t.Fatal("freeze changed the caller input or dispatched work")
							}
						})
					}
				}
			}
			for _, field := range []string{"executable", "model", "reasoning_effort"} {
				for _, value := range []any{"", nil} {
					t.Run(fmt.Sprintf("reject-%s/%v", field, value), func(t *testing.T) {
						// Keep field presence through map/raw JSON: CodexSettings's
						// omitempty would erase the exact negative input being tested.
						settings := map[string]any{"access_mode": mode, "executable": executable}
						input := marshal(t, settings)
						if fault := gate(t, input); fault != nil {
							t.Fatal("invalid positive fixture", fault)
						}
						settings[field] = value
						input = marshal(t, settings)
						before := bytes.Clone(input.Input)
						fault := gate(t, input)
						if fault == nil || fault.Exit != 2 || fault.Detail.Code != "invalid_input" {
							t.Fatal("explicit empty/null field accepted by gate", fault)
						}
						dir := filepath.Join(t.TempDir(), "must-not-be-created")
						r, err := freeze(context.Background(), dir, input)
						if r != nil || !errors.As(err, &fault) || fault.Exit != 2 || fault.Detail.Code != "invalid_input" {
							t.Fatal("invalid field reached admission", r, err)
						}
						if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
							t.Fatal("invalid request performed preparation I/O", err)
						}
						if !bytes.Equal(before, input.Input) {
							t.Fatal("negative input was rewritten")
						}
					})
				}
			}
		})
	}
}

func TestCodexDiagnosticPublicationUsesExistingRedaction(t *testing.T) {
	t.Setenv("GH_TOKEN", "fixture-codex-known-token")
	t.Setenv("CCR_TEST_AUTH", "fixture-codex-referenced-token")
	e, w, _ := operationEngine(t)
	r := e.runs["run"].Clone()
	r.Request.Execution.Adapter = "codex-app-server"
	r.Request.Publication.Settings = json.RawMessage(`{"authentication_refs":["CCR_TEST_AUTH"]}`)
	if err := e.commitLocked(r, nil, "codex_diagnostic_fixture"); err != nil {
		t.Fatal(err)
	}
	e.block("run", "preparation_unverified", errors.New("original native failure: fixture-codex-known-token / fixture-codex-referenced-token"), false)
	want := "original native failure: [REDACTED] / [REDACTED]"
	response := e.Handle("inspect", "run", "", "", "", 0, nil)
	if !response.OK {
		t.Fatal(response)
	}
	view := response.Data.(map[string]any)["run"].(contracts.RunView)
	if view.Diagnostic != want || view.Phase != "preparation_unverified" || view.Status != "blocked" {
		t.Fatal("public diagnostic changed meaning or leaked credentials", view)
	}
	stored, err := e.store.Runs()
	if err != nil || len(stored) != 1 {
		t.Fatal("diagnostic was not committed", err)
	}
	if stored[0].Diagnostic != want || stored[0].Phase != view.Phase || stored[0].Status != view.Status || stored[0].Allowed || !stored[0].ResumeRequired {
		t.Fatal("persisted diagnostic or blocking semantics differ", stored[0].Diagnostic)
	}
	port := w.port.(*operationCommitPort)
	if port.executes != 0 || port.reconciles != 0 || len(stored[0].Intents) != 0 {
		t.Fatal("diagnostic publication dispatched native work")
	}
}
