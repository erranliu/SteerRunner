package orca

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/coder/websocket"
)

// StateConnectionError marks a failed read-only connection, never an agent send.
// Keep the cause for classification without exposing pairing URLs or credentials.
type StateConnectionError struct {
	Cause  error
	Status int
}

func (e *StateConnectionError) Error() string {
	reason := "transport unavailable"
	var dns *net.DNSError
	switch {
	case e.Status != 0:
		reason = fmt.Sprintf("HTTP %d", e.Status)
	case errors.Is(e.Cause, context.DeadlineExceeded):
		reason = "timeout"
	case errors.Is(e.Cause, context.Canceled):
		reason = "canceled"
	case errors.As(e.Cause, &dns):
		reason = "DNS lookup failed"
	case errors.Is(e.Cause, syscall.ECONNREFUSED):
		reason = "connection refused"
	case errors.Is(e.Cause, syscall.ECONNRESET):
		reason = "connection reset"
	default:
		var network net.Error
		if errors.As(e.Cause, &network) && network.Timeout() {
			reason = "timeout"
		}
	}
	return "Orca state streaming connection failed: " + reason
}

func (e *StateConnectionError) Unwrap() error { return e.Cause }

func IsStateConnectionError(err error) bool {
	var connection *StateConnectionError
	return errors.As(err, &connection)
}

func stateConnectionFailure(err error) error {
	var certificate *tls.CertificateVerificationError
	if errors.As(err, &certificate) {
		return errors.New("Orca state TLS certificate verification failed")
	}
	switch status := websocket.CloseStatus(err); status {
	case websocket.StatusPolicyViolation, websocket.StatusProtocolError, websocket.StatusUnsupportedData, websocket.StatusInvalidFramePayloadData:
		return fmt.Errorf("Orca state connection rejected: close status %d", status)
	}
	return &StateConnectionError{Cause: err}
}
