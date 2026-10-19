package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type ControlClosure struct {
	Process *b.ProcessIdentity `json:"process"`
	Closed  bool               `json:"closed"`
}

var errAuthenticationProofUnverified = errors.New("original authentication revocation remains unverified")

func validateAuthenticationStop(ready codex.RemoteReadiness, proof codex.RemoteAuthenticationCleanup) error {
	a, _ := json.Marshal(ready)
	b, _ := json.Marshal(proof.Readiness)
	if string(a) != string(b) || !proof.Revoked || !proof.BrokerClosed || !proof.SocketCleaned || proof.Mode != "host-gh-repository-broker" && proof.Mode != "github-app-repository-broker" {
		return errAuthenticationProofUnverified
	}
	return nil
}

// ReadAuthenticationStopEvidence only reads bound original transport evidence.
func ReadAuthenticationStopEvidence(directory string, snapshot b.LifecycleSnapshot, state State) (codex.RemoteAuthenticationCleanup, ControlClosure, error) {
	var empty codex.RemoteAuthenticationCleanup
	if state.RemoteReadiness == nil || state.Control == nil {
		return empty, ControlClosure{}, errAuthenticationProofUnverified
	}
	l := &Lifecycle{setup: b.LifecycleSetup{RunDirectory: directory, Snapshot: snapshot}}
	j, err := l.loadControlCheckpoint(state)
	if err != nil || !j.Closed || j.Request.Method != "authentication-stop" || j.Response == nil || j.Response.Error != "" || j.Response.Authentication == nil || validateAuthenticationStop(*state.RemoteReadiness, *j.Response.Authentication) != nil {
		return empty, ControlClosure{}, errors.Join(err, errAuthenticationProofUnverified)
	}
	return *j.Response.Authentication, ControlClosure{Process: &state.Control.Process, Closed: true}, nil
}

func (l *Lifecycle) revokeAuthentication(ctx context.Context, state State) error {
	return l.reconcileAuthenticationStop(ctx, state)
}

func (l *Lifecycle) reconcileAuthenticationStop(ctx context.Context, state State) error {
	if state.RemoteReadiness == nil || !state.RemoteReadiness.AuthenticationControl {
		return nil
	}
	release, err := l.inspectionGate(ctx)
	if err != nil {
		return err
	}
	defer release()
	j, err := l.loadControlCheckpoint(state)
	if err != nil {
		return err
	}
	if j.Response != nil && j.Request.Method == "authentication-stop" && j.Response.Authentication != nil && j.Response.Error == "" {
		return validateAuthenticationStop(*state.RemoteReadiness, *j.Response.Authentication)
	}
	r, err := l.callControl(ctx, state, "authentication-stop", "")
	if err != nil {
		return err
	}
	if r.Error != "" || r.Authentication == nil {
		return errAuthenticationProofUnverified
	}
	return validateAuthenticationStop(*state.RemoteReadiness, *r.Authentication)
}
