package application

import (
	"bytes"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestServiceGenerationContentDoesNotRequireUnusedParserConfiguration(t *testing.T) {
	for _, flow := range []string{"conversation", "generate"} {
		t.Run(flow, func(t *testing.T) {
			e, _ := contentEngine(t, flow)
			r := e.runs["run"].Clone()
			r.AcquisitionConfig = ra.FrozenConfig{}
			before := canonical(r)
			items := TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), workflow.Catalog())
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
			items := TakeoverFacts([]*runstate.Record{r}, resources.RecoveryCapabilities(), workflow.Catalog())
			if len(items) != 1 || items[0].Status != "unsupported" || !bytes.Equal(before, canonical(r)) {
				t.Fatal("retired auth silently adopted current gh identity", items)
			}
		})
	}
}
