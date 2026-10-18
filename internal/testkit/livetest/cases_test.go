package livetest

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistryPreservesReliabilityAndBindsControlledEdges(t *testing.T) {
	reliability, edges := 0, 0
	seen := map[string]bool{}
	for _, scenario := range Scenarios() {
		cases := map[string]bool{}
		for _, c := range scenario.Cases {
			if c.ID == "" || c.Outcome == "" || cases[c.ID] {
				t.Fatal("empty or duplicate case", scenario.ID, c.ID)
			}
			cases[c.ID] = true
			if scenario.Driver == "execution-stability" {
				if scenario.Level != "L3" || scenario.Target != "orca-environment" || c.Test != "" {
					t.Fatal("core reliability changed level or target", scenario.ID)
				}
				reliability++
			}
			if scenario.Level == "L4" {
				if scenario.Driver != "controlled-integration" || scenario.Target != "controlled" || !strings.HasPrefix(c.Test, "TestSREdge") || seen[c.Test] {
					t.Fatal("edge has missing, duplicate or unsafe test binding", scenario.ID)
				}
				seen[c.Test] = true
				edges++
			} else if c.Test != "" {
				t.Fatal("native case bound to a controlled substitute", scenario.ID)
			}
		}
	}
	if reliability != 12 || edges != 4 {
		t.Fatalf("core/edge coverage changed: reliability=%d edges=%d", reliability, edges)
	}
}

func TestLiveRegistryHasCompleteL3Cases(t *testing.T) {
	scenarios, cases := 0, 0
	seen := map[string]bool{}
	for _, s := range Scenarios() {
		if seen[s.ID] {
			t.Fatal("duplicate scenario")
		}
		seen[s.ID] = true
		if s.Level != "L3" {
			continue
		}
		scenarios++
		for _, c := range s.Cases {
			cases++
			if _, _, err := Lookup(s.ID, c.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if scenarios != 17 || cases != 24 {
		t.Fatalf("expected seventeen scenarios/twenty-four cases, got %d/%d", scenarios, cases)
	}
	for _, v := range [][2]string{{"orca-remote.smoke", "kill-active"}, {"unknown", "default"}, {"orca-remote.service-restart", ""}} {
		if _, _, err := Lookup(v[0], v[1]); err == nil {
			t.Fatal("unknown case accepted")
		}
	}
}
func TestStabilityResultCannotPassMissingOrMismatchedEvidence(t *testing.T) {
	m := Manifest{Scenario: "orca-remote.caller-exit", CaseID: "accepted-response-lost", RepeatIndex: 1}
	original := StabilityResult{Scenario: m.Scenario, CaseID: m.CaseID, RepeatIndex: 1, RunID: "original", TestOutcome: "passed", BusinessOutcome: "merged", FaultsHit: 1, AuditComplete: true, PRURL: "https://github.com/erranliu/SteerlineTest/pull/1", MergeCommit: strings40, Cleanup: Cleanup{NativeStopped: true, ServiceStopped: true, FacilitiesRestored: true}, Assertions: []Assertion{{Name: "fault-hit", Passed: true, Evidence: "faults.jsonl"}, {Name: "original-identity", Passed: true, Evidence: "native.json"}, {Name: "no-duplicate-inputs", Passed: true, Evidence: "calls.jsonl"}}}
	if err := original.Validate(m); err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*StabilityResult){"no-fault": func(r *StabilityResult) { r.FaultsHit = 0 }, "no-audit": func(r *StabilityResult) { r.AuditComplete = false }, "wrong-case": func(r *StabilityResult) { r.CaseID = "default" }, "wrong-repeat": func(r *StabilityResult) { r.RepeatIndex = 2 }, "dynamic-outcome": func(r *StabilityResult) { r.BusinessOutcome = "revoked-unverified" }, "no-native-stop": func(r *StabilityResult) { r.Cleanup.NativeStopped = false }, "no-facility-restore": func(r *StabilityResult) { r.Cleanup.FacilitiesRestored = false }, "no-merge": func(r *StabilityResult) { r.MergeCommit = "" }, "unproven": func(r *StabilityResult) { r.Assertions[0].Evidence = "" }, "duplicate": func(r *StabilityResult) { r.Assertions[1] = r.Assertions[0] }, "missing-contract": func(r *StabilityResult) { r.Assertions[1].Name = "anything" }, "failed-assertion": func(r *StabilityResult) { r.Assertions[0].Passed = false }}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(original)
			var r StabilityResult
			_ = json.Unmarshal(raw, &r)
			mutate(&r)
			if r.Validate(m) == nil {
				t.Fatal("false success accepted")
			}
		})
	}
}
