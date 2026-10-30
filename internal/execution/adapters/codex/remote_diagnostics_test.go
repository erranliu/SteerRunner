package codexworker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRemoteAuthenticationFailureDiagnosticsRemainFinite(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{errors.New("remote_authentication_helper_mismatch: private-path-secret"), "remote_authentication_helper_mismatch"},
		{context.DeadlineExceeded, "remote_authentication_stop_deadline"},
		{context.Canceled, "remote_authentication_stop_canceled"},
		{&os.PathError{Op: "open", Path: "credential-bearing-private-path", Err: os.ErrPermission}, "remote_authentication_stop_permission_denied"},
		{errors.New("Authorization: Bearer secret-value"), "remote_authentication_stop_unverified"},
	} {
		got := RemoteAuthenticationDiagnostic(test.err)
		if got != test.want || strings.Contains(got, "secret") || RemoteAuthenticationDiagnostic(errors.New(got)) != got {
			t.Fatal("nonfinite or unstable diagnostic", got)
		}
	}
}

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
