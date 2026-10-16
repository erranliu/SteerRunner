package livetest

import "testing"

func TestStabilityEvidenceRejectsFlagOnlySuccess(t *testing.T) {
	m := Manifest{Root: t.TempDir(), Scenario: "orca-remote.caller-exit", CaseID: "accepted-response-lost", RepeatIndex: 1}
	r := StabilityResult{Scenario: m.Scenario, CaseID: m.CaseID, RepeatIndex: 1, RunID: "original", TestOutcome: "passed", BusinessOutcome: "merged", FaultsHit: 1, AuditComplete: true, PRURL: "https://github.com/erranliu/SteerlineTest/pull/1", MergeCommit: strings40, Cleanup: Cleanup{NativeStopped: true, ServiceStopped: true, FacilitiesRestored: true}, Assertions: []Assertion{{Name: "fault-hit", Passed: true, Evidence: "claimed"}, {Name: "original-identity", Passed: true, Evidence: "claimed"}, {Name: "no-duplicate-inputs", Passed: true, Evidence: "claimed"}}}
	if err := r.Validate(m); err != nil {
		t.Fatal("shape guard unexpectedly rejected complete claims", err)
	}
	if ValidateEvidence(m, r) == nil {
		t.Fatal("claimed passing flags without physical evidence were accepted")
	}
}
