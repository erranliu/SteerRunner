package codexworker

import (
	"errors"
	"testing"
)

func TestEnvironmentObservationDiagnosticsPreserveFiniteOriginalFailure(t *testing.T) {
	for _, code := range []string{"original_native_turn_unsettled", "authentication_broker_unavailable", "authentication_api_binding_invalid", "authentication_expired_or_unavailable"} {
		if got := EnvironmentDiagnostic(errors.New(code + ": credential-bearing-provider-detail")); got != code {
			t.Fatal("original failure category lost", got)
		}
	}
	if got := EnvironmentDiagnostic(errors.New("Authorization: Bearer secret-value")); got != "preparation_unverified" {
		t.Fatal("provider detail escaped", got)
	}
}
