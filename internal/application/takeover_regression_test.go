package application

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestServiceGenerationCheckRejectsRetiredAuthenticationAndUnknownOperations(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		for _, scenario := range []string{"current", "null-auth", "app-auth", "unknown-operation"} {
			t.Run(adapter+"/"+scenario, func(t *testing.T) {
				settings := `{"access_mode":"sandbox"}`
				if scenario == "null-auth" {
					settings = `{"access_mode":"sandbox","github_auth":null}`
				}
				if scenario == "app-auth" {
					settings = `{"access_mode":"sandbox","github_auth":{"app_id":1,"installation_id":2,"private_key_env":"OLD_KEY"}}`
				}
				r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original-check", Revision: 7, InputDigest: "original-input", Input: contracts.CheckInput{Execution: contracts.Execution{Adapter: adapter, Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(settings)}}}}
				if scenario == "unknown-operation" {
					r.Operations = []b.Operation{{Version: 99}}
				}
				before := canonical(r)
				items := TakeoverFacts(nil, []*runstate.CheckRecord{r}, resources.RecoveryCapabilities(), workflow.Catalog())
				want := "unsupported"
				if scenario == "current" {
					want = "compatible"
				}
				if len(items) != 1 || items[0].Status != want || items[0].ID != r.View.CheckID || items[0].Revision != r.View.Revision || items[0].InputDigest != r.View.InputDigest || !bytes.Equal(before, canonical(r)) {
					t.Fatal("takeover accepted an unsupported Check or changed original facts", items)
				}
			})
		}
	}
}

func TestServiceGenerationContentDoesNotRequireUnusedParserConfiguration(t *testing.T) {
	for _, flow := range []string{"conversation", "generate"} {
		t.Run(flow, func(t *testing.T) {
			e, _ := contentEngine(t, flow)
			r := e.runs["run"].Clone()
			r.AcquisitionConfig = ra.FrozenConfig{}
			before := canonical(r)
			items := TakeoverFacts([]*runstate.Record{r}, nil, resources.RecoveryCapabilities(), workflow.Catalog())
			if len(items) != 1 || items[0].Status != "compatible" || !bytes.Equal(before, canonical(r)) {
				t.Fatal("content takeover required unused parser defaults or rewrote frozen config", items)
			}
		})
	}
}

func TestServiceGenerationRunRejectsRetiredFrozenProfileWithoutChangingFacts(t *testing.T) {
	for _, auth := range []string{`null`, `{}`, `{"app_id":1,"installation_id":2,"private_key_env":"OLD_KEY"}`} {
		t.Run(auth, func(t *testing.T) {
			e, _ := contentEngine(t, "generate")
			r := e.runs["run"].Clone()
			for i := range r.Profile {
				payload := strings.TrimSuffix(string(r.Profile[i].Payload), "}") + `,"github_auth":` + auth + `}`
				var err error
				r.Profile[i], err = b.NewImplementationDataEnvelope(r.Profile[i], []byte(payload))
				if err != nil {
					t.Fatal(err)
				}
			}
			before := canonical(r)
			items := TakeoverFacts([]*runstate.Record{r}, nil, resources.RecoveryCapabilities(), workflow.Catalog())
			if len(items) != 1 || items[0].Status != "unsupported" || !bytes.Equal(before, canonical(r)) {
				t.Fatal("retired auth silently adopted current gh identity", items)
			}
		})
	}
}
