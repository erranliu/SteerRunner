package contracts

import "testing"

func TestPendingStopsStrictOriginalEnvelope(t *testing.T) {
	valid := `{"pending_stops":[{"run_id":"run","request":{"protocol":"steerrunner-cli/v1","request_id":"stop","input":{"expected_revision":3}}}]}`
	if _, f := ReadServiceStartInput([]byte(valid)); f != nil {
		t.Fatal(f)
	}
	for _, body := range []string{`{}`, `{"pending_stops":null}`, `{"pending_stops":[],"release":true}`, `{"pending_stops":[{"run_id":"run","request":{"protocol":"steerrunner-cli/v1","request_id":"stop","input":{"expected_revision":0}}}]}`, valid + `{}`} {
		if _, f := ReadServiceStartInput([]byte(body)); f == nil {
			t.Fatalf("unsupported startup input accepted: %s", body)
		}
	}
}
