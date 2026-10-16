package codexworker

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestOriginalPRVerificationSurvivesMissingListIndexAndRejectsChangedObjects(t *testing.T) {
	s := &EnvironmentSession{setup: EnvironmentSetup{Nonce: "original-nonce"}, gate: &checkWriteGate{base: "original/base", head: "original/head"}, backend: Backend{Config: Config{Authentication: &AuthenticationRuntime{repository: "erranliu/SteerlineTest"}}}}
	for _, scenario := range []string{"valid", "wrong-number", "wrong-url", "wrong-head", "wrong-base-repo", "wrong-body", "merged", "unresolved-number"} {
		t.Run(scenario, func(t *testing.T) {
			resources := contracts.CheckResources{PRNumber: 356, BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40)}
			pr := map[string]any{"number": 356, "html_url": "https://github.com/erranliu/SteerlineTest/pull/356", "state": "open", "body": "original-nonce", "base": map[string]any{"sha": resources.BaseSHA, "ref": "original/base", "repo": map[string]string{"full_name": "erranliu/SteerlineTest"}}, "head": map[string]any{"sha": resources.HeadSHA, "ref": "original/head", "repo": map[string]string{"full_name": "erranliu/SteerlineTest"}}}
			switch scenario {
			case "wrong-number":
				pr["number"] = 357
			case "wrong-url":
				pr["html_url"] = "https://github.com/foreign/repo/pull/356"
			case "wrong-head":
				pr["head"].(map[string]any)["sha"] = strings.Repeat("c", 40)
			case "wrong-base-repo":
				pr["base"].(map[string]any)["repo"] = map[string]string{"full_name": "foreign/repo"}
			case "wrong-body":
				pr["body"] = "foreign-nonce"
			case "merged":
				pr["merged_at"] = "2026-10-16T00:00:00Z"
			case "unresolved-number":
				resources.PRNumber = 0
			}
			body, err := s.probeScript("pr_query", resources)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(pr)
			// Model a lagging list index while the original numbered object exists.
			script := `$ErrorActionPreference='Stop'; function API { if ($args -contains 'repos/erranliu/SteerlineTest/pulls') { return @() }; if ($args -notcontains 'repos/erranliu/SteerlineTest/pulls/356') { throw 'unexpected_request' }; return (` + psLiteral(string(raw)) + `|ConvertFrom-Json) }; $facts=@{}; ` + body + `; $facts|ConvertTo-Json -Compress`
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
			if scenario == "valid" {
				var facts map[string]string
				if err != nil || json.Unmarshal(output, &facts) != nil || facts["pr_number"] != "356" {
					t.Fatalf("original object was not verified: %s %v", output, err)
				}
			} else if err == nil {
				t.Fatal("foreign or unresolved object was accepted", scenario)
			}
		})
	}
}
