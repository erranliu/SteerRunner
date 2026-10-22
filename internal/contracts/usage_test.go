package contracts

import "testing"

func TestUsageAuthorizationFenceInput(t *testing.T) {
	valid := `{"pending_stops":[],"usage_policy":{"enabled":true,"remote_log_access":{"mode":"disabled","grants":[]}}}`
	v, f := ReadServiceStartInput([]byte(valid))
	if f != nil || v.UsagePolicy == nil {
		t.Fatal(v, f)
	}
	for _, raw := range []string{`{"pending_stops":[],"usage_policy":null}`, `{"pending_stops":[],"usage_policy":{"remote_log_access":{"mode":"disabled","grants":[]}}}`, `{"pending_stops":[],"usage_policy":{"enabled":null,"remote_log_access":{"mode":"disabled","grants":[]}}}`, `{"pending_stops":[],"usage_policy":{"enabled":true,"remote_log_access":{"mode":"temporary","grants":null}}}`, `{"pending_stops":[],"usage_policy":{"enabled":true,"remote_log_access":{"mode":"disabled","grants":[]},"cost":{}}}`} {
		if _, f = ReadServiceStartInput([]byte(raw)); f == nil {
			t.Fatal("invalid policy accepted", raw)
		}
	}
	for _, code := range []string{"snapshot_not_found", "snapshot_expired", "invalid_usage_cursor", "usage_policy_conflict"} {
		if got := ExitCode(Rejected("", Fail(5, code, "test"))); got != 5 {
			t.Fatal(code, got)
		}
	}
}
