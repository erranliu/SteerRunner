package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func checkInputFixture() CheckInput {
	return CheckInput{Repository: Repository{URL: "https://github.com/owner/repo.git", BaseRevision: strings.Repeat("a", 40), Remote: "origin", BaseBranch: "main"}, Execution: Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"sandbox"}`)}}
}

func TestEnvironmentCheckInputAndManifest(t *testing.T) {
	value := checkInputFixture()
	raw, _ := json.Marshal(value)
	if f := ValidateEnvironmentCheck(raw); f != nil {
		t.Fatal(f)
	}
	for _, bad := range []string{`{"task":"x"}`, `{"documents":{}}`, `{"execution_timeout":"0s"}`, `{"execution_timeout":"2h"}`, `{"cleanup_timeout":"-1s"}`} {
		var delta map[string]any
		_ = json.Unmarshal([]byte(bad), &delta)
		var object map[string]any
		_ = json.Unmarshal(raw, &object)
		for k, v := range delta {
			object[k] = v
		}
		data, _ := json.Marshal(object)
		if ValidateEnvironmentCheck(data) == nil {
			t.Fatal("invalid input accepted", bad)
		}
	}
	for _, mode := range []string{"sandbox", "full_access"} {
		value.Execution.Settings = json.RawMessage(`{"access_mode":"` + mode + `"}`)
		data, _ := json.Marshal(value)
		if f := ValidateEnvironmentCheck(data); f != nil {
			t.Fatal(f)
		}
	}
	for _, mode := range []string{"", "auto", "FULL_ACCESS"} {
		value.Execution.Settings = json.RawMessage(`{"access_mode":"` + mode + `"}`)
		data, _ := json.Marshal(value)
		if mode != "" && ValidateEnvironmentCheck(data) == nil {
			t.Fatal(mode)
		}
	}
	if len(CheckManifest(true)) == 0 {
		t.Fatal("empty manifest")
	}
	for _, item := range CheckManifest(false) {
		if strings.HasPrefix(item.ID, "pr_") && (item.Required || item.Status != "not_applicable") {
			t.Fatal(item)
		}
	}
	if ExitCode(Rejected("", Fail(5, "check_not_found", "missing"))) != 5 {
		t.Fatal("exit mapping")
	}
}

func TestEnvironmentCheckEvidenceAndRevisionGuards(t *testing.T) {
	c := EnvironmentCheck{CheckID: "original", Revision: 2, Input: checkInputFixture(), InputDigest: "input", ResourceDigest: "resources", Actual: CheckContext{SessionID: "session", TurnID: "turn", Nonce: "nonce"}, Items: CheckManifest(false)}
	if CheckResult(c) != "unverified" {
		t.Fatal("missing evidence passed")
	}
	for i := range c.Items {
		if c.Items[i].Required {
			c.Items[i].Status = "passed"
			c.Items[i].Evidence = []CheckEvidence{{CheckID: c.CheckID, SessionID: "session", TurnID: "turn", Nonce: "nonce", OperationID: "op", ToolCallID: "tool", Verified: true, Source: "native_tool_and_owner_verification"}}
		}
	}
	if CheckResult(c) != "passed" {
		t.Fatal("valid evidence rejected")
	}
	c.Items[0].Evidence[0].Source = "model_json"
	if CheckResult(c) != "unverified" {
		t.Fatal("model prose passed")
	}
	c.Items[0].Status = "failed"
	if CheckResult(c) != "failed" {
		t.Fatal("failure hidden")
	}
	for _, mutate := range []func(*EnvironmentCheck){func(v *EnvironmentCheck) { v.Revision-- }, func(v *EnvironmentCheck) { v.InputDigest = "changed" }, func(v *EnvironmentCheck) { v.ResourceDigest = "changed" }, func(v *EnvironmentCheck) { v.Input.Repository.BaseBranch = "changed" }, func(v *EnvironmentCheck) { v.Status = "different_same_revision" }} {
		raw, _ := json.Marshal(c)
		var next EnvironmentCheck
		_ = json.Unmarshal(raw, &next)
		mutate(&next)
		if ValidateCheckUpdate(c, next) == nil {
			t.Fatal("conflicting snapshot accepted")
		}
	}
	next := c
	next.Revision++
	if ValidateCheckUpdate(c, next) != nil {
		t.Fatal("newer snapshot rejected")
	}
}
