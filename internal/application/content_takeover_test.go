package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestContentFrozenTakeoverContracts(t *testing.T) {
	for _, id := range []string{"generate", "conversation"} {
		t.Run(id, func(t *testing.T) {
			dir := t.TempDir()
			// An unusable mutable development config must affect neither content
			// admission nor takeover; no defaults are added to the original record.
			if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(dir, "service", "result-acquisition.json")
			if err := os.WriteFile(configPath, []byte(`{"schema":"unsupported"}`), 0600); err != nil {
				t.Fatal(err)
			}
			material := "original submitted bytes"
			docs := []contracts.Document{{Kind: "inline", Path: "source.txt", Content: &material}}
			request := contracts.RunRequest{Title: "Content takeover", Workflow: contracts.Workflow{ID: id, Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: canonical(contracts.CodexSettings{Executable: os.Args[0], AccessMode: "read_only"})}}
			if id == "generate" {
				request.Generate = &contracts.GenerateInput{Instruction: "Return content", Materials: docs}
			} else {
				request.Conversation = &contracts.ConversationInput{Instruction: "Discuss", Materials: docs, Directory: contracts.DirectoryAccess{Path: t.TempDir(), Access: "read_only"}}
			}
			input := contracts.Request{Protocol: contracts.Protocol, RequestID: "original-submit", Input: canonical(request)}
			if err := contracts.ValidateRun(input.Input); err != nil {
				t.Fatal(err)
			}
			r, err := freeze(context.Background(), dir, input)
			if err != nil {
				t.Fatal(err)
			}
			if r.AcquisitionConfig != (ra.FrozenConfig{}) {
				t.Fatal("content admission manufactured development config")
			}
			s, err := storage.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			command := storage.NewCommand("submit", r.ID, input)
			r.Revision = 1
			if err := s.Commit(r, 0, &command); err != nil {
				t.Fatal(err)
			}
			check := func(record *runstate.Record, want string) contracts.TakeoverItem {
				t.Helper()
				before := canonical(record)
				item := s.BindTakeoverRequests(TakeoverFacts([]*runstate.Record{record}, resources.RecoveryCapabilities(), workflow.Catalog()))[0]
				if item.Status != want || !bytes.Equal(before, canonical(record)) || item.ID != r.ID || item.InputDigest != r.InputDigest || item.RequestID != input.RequestID || item.RequestDigest != command.Digest {
					t.Fatalf("takeover lost original facts or wrong status: %+v", item)
				}
				return item
			}
			check(r, "compatible")
			if err := os.WriteFile(configPath, []byte("changed invalid mutable config"), 0600); err != nil {
				t.Fatal(err)
			}
			check(r, "compatible")
			for _, variant := range []string{"codec", "workflow", "workflow-version", "semantics", "execution-record", "material", "parser"} {
				t.Run(variant, func(t *testing.T) {
					copy := r.Clone()
					switch variant {
					case "codec":
						copy.Profile[0].CodecVersion = 99
						copy.Profile[0], err = b.NewImplementationDataEnvelope(copy.Profile[0], copy.Profile[0].Payload)
						if err != nil {
							t.Fatal(err)
						}
					case "workflow":
						copy.Request.Workflow.ID = "unknown"
					case "workflow-version":
						copy.Request.Workflow.Version = 99
					case "semantics":
						copy.Workflow.Config.Initial = "unknown"
					case "execution-record":
						copy.Snapshot.Ref.Version = 99
					case "material":
						copy.Materials[0].Content = "replaced"
					case "parser":
						copy.Intents = []operation.Intent{{Request: b.LifecycleRequest{Operation: b.Operation{Version: 1}, Input: b.LifecycleInput{Parse: &b.ParseInput{Payload: `{}`}}}}}
					}
					check(copy, "unsupported")
				})
			}
			stored, err := s.Runs()
			if err != nil || len(stored) != 1 || !bytes.Equal(canonical(r), canonical(stored[0])) {
				t.Fatal("takeover changed stored record", err)
			}
		})
	}
}

func TestTakeoverActualParserContractStillRequired(t *testing.T) {
	e, _, index := parserEngine(t)
	content, _ := contentEngine(t, "generate")
	r := content.runs["run"].Clone()
	r.AcquisitionConfig = e.runs["run"].AcquisitionConfig
	r.Intents = []operation.Intent{e.runs["run"].Intents[index]}
	index = 0
	// Keep a real frozen parser operation while selecting a final-only profile:
	// workflow capability cannot hide an existing parser's persisted contract.
	if err := parser.ValidateRecovery(r.Intents[index].Request); err != nil {
		t.Fatal("original parser fixture invalid", err)
	}
	if item := TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), workflow.Catalog())[0]; item.Status != "compatible" {
		t.Fatal(item)
	}
	var pack map[string]any
	if err := json.Unmarshal([]byte(r.Intents[index].Request.Input.Parse.Payload), &pack); err != nil {
		t.Fatal(err)
	}
	pack["profile"] = "unknown-parser-profile"
	r.Intents[index].Request.Input.Parse.Payload = string(canonical(pack))
	before := canonical(r)
	item := TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), workflow.Catalog())[0]
	if item.Status != "unsupported" || item.Diagnostic != "frozen parser/result acquisition contract unsupported" || !bytes.Equal(before, canonical(r)) {
		t.Fatal(item)
	}
}
