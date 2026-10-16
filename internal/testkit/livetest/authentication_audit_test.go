package livetest

import (
	"strings"
	"testing"
)

func TestBrokerCleanupRejectsHiddenRefusalsAndMissingFacts(t *testing.T) {
	good := `{"revocation_confirmed":true,"broker":{"closed":true,"denied_requests":0,"socket_cleanup_confirmed":true}}`
	for _, c := range []struct {
		name, body string
		ok         bool
	}{
		{"settled-without-refusal", good, true},
		{"hidden-refusal", strings.Replace(good, `"denied_requests":0`, `"denied_requests":1`, 1), false},
		{"missing-counter", strings.Replace(good, `"denied_requests":0,`, "", 1), false},
		{"not-closed", strings.Replace(good, `"closed":true`, `"closed":false`, 1), false},
		{"socket-remains", strings.Replace(good, `"socket_cleanup_confirmed":true`, `"socket_cleanup_confirmed":false`, 1), false},
		{"not-revoked", strings.Replace(good, `"revocation_confirmed":true`, `"revocation_confirmed":false`, 1), false},
		{"missing-broker", `{"revocation_confirmed":true}`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := CheckBrokerCleanup([]byte(c.body)); (err == nil) != c.ok {
				t.Fatalf("unexpected cleanup qualification: %v", err)
			}
		})
	}
}
