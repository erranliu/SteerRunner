package orca

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func TestStateConnectionFailureClassification(t *testing.T) {
	for _, status := range []int{401, 403, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			_, err := openStateSocket(context.Background(), statePairing{Endpoint: "ws" + strings.TrimPrefix(server.URL, "http") + "/?secret=canary", Token: "secret-canary"}, "runtime")
			if err == nil || IsStateConnectionError(err) != (status == 429 || status >= 500) || strings.Contains(err.Error(), "canary") || strings.Contains(err.Error(), server.Listener.Addr().String()) {
				t.Fatal("unsafe failure classification", err)
			}
		})
	}
	root := &url.Error{Op: "dial", URL: "wss://private.example/?token=canary", Err: context.DeadlineExceeded}
	err := &StateConnectionError{Cause: root}
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "canary") {
		t.Fatal("cause lost or leaked", err)
	}
}

func TestStateConnectionRejectionIsNotRetryable(t *testing.T) {
	for _, err := range []error{
		&tls.CertificateVerificationError{Err: errors.New("private certificate canary")},
		websocket.CloseError{Code: websocket.StatusPolicyViolation, Reason: "credential canary"},
		websocket.CloseError{Code: websocket.StatusProtocolError, Reason: "private frame canary"},
	} {
		classified := stateConnectionFailure(err)
		if IsStateConnectionError(classified) || strings.Contains(classified.Error(), "canary") {
			t.Fatal("verification rejection treated as transient or leaked", classified)
		}
	}
}
