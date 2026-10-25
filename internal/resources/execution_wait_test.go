package resources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestExecutionWaitPublicSchema(t *testing.T) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(Read("execution-wait.schema.json"))))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err = c.AddResource("https://runner.invalid/wait", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("https://runner.invalid/wait")
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(nil); err != nil {
		t.Fatal("explicit null rejected", err)
	}
	now := time.Now().UTC()
	for _, stage := range []string{"queue", "calling", "backoff", "exhausted"} {
		v := b.ExecutionWait{Contract: b.ExecutionWaitContract, OperationID: "original", Query: "runtime_status", Stage: stage, StartedAt: now, UpdatedAt: now, DeadlineAt: now.Add(time.Minute), MaxCalls: 5, Diagnostic: "原始原因"}
		switch stage {
		case "queue":
			v.ReasonCode = "orca_cli_busy"
		case "calling":
			v.ReasonCode = "orca_query_running"
			v.CallsStarted = 1
		case "backoff":
			v.ReasonCode = "orca_query_transient"
			v.CallsStarted = 1
			next := now.Add(time.Second)
			v.NextAttemptAt = &next
		case "exhausted":
			v.ReasonCode = "wait_budget_exhausted"
		}
		body, _ := json.Marshal(contracts.RunView{ExecutionWait: &v})
		var view map[string]any
		_ = json.Unmarshal(body, &view)
		if err = schema.Validate(view["execution_wait"]); err != nil {
			t.Fatal(stage, err)
		}
		for _, mutate := range []func(map[string]any){func(m map[string]any) { m["contract"] = "other" }, func(m map[string]any) { m["max_calls"] = 6 }, func(m map[string]any) { m["unknown"] = "extra" }, func(m map[string]any) { m["reason_code"] = "not-a-reason" }, func(m map[string]any) { m["next_attempt_at"] = 42 }} {
			var m map[string]any
			raw, _ := json.Marshal(v)
			_ = json.Unmarshal(raw, &m)
			mutate(m)
			if schema.Validate(m) == nil {
				t.Fatal("invalid public fact accepted", m)
			}
		}
	}
	raw, _ := json.Marshal(Capabilities())
	var caps map[string]json.RawMessage
	_ = json.Unmarshal(raw, &caps)
	var cap struct {
		Contract     string
		SameSnapshot bool   `json:"same_snapshot"`
		Property     string `json:"run_property"`
		Schema       json.RawMessage
	}
	if json.Unmarshal(caps["execution_wait"], &cap) != nil || cap.Contract != b.ExecutionWaitContract || !cap.SameSnapshot || cap.Property != "execution_wait" {
		t.Fatal("capability missing", string(raw))
	}
	var advertised any
	_ = json.Unmarshal(cap.Schema, &advertised)
	if raw, _ := json.Marshal(advertised); strings.Contains(string(raw), `"$ref":"https:`) {
		t.Fatal("remote schema reference")
	}
}
