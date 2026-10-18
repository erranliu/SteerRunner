package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"strings"
	"time"
)

type authenticationStopJournal struct {
	Failure string                             `json:"failure,omitempty"`
	Binding string                             `json:"binding"`
	Attempt NativeInspection                   `json:"attempt"`
	Proof   *codex.RemoteAuthenticationCleanup `json:"proof,omitempty"`
}

var errAuthenticationProofUnverified = errors.New("original authentication revocation remains unverified")

// ReadAuthenticationStopEvidence verifies caller-owned cleanup evidence without
// contacting or recreating any remote actor.
func ReadAuthenticationStopEvidence(directory string, snapshot b.LifecycleSnapshot, state State) (codex.RemoteAuthenticationCleanup, NativeInspection, error) {
	l := &Lifecycle{setup: b.LifecycleSetup{RunDirectory: directory, Snapshot: snapshot}}
	path, binding, err := l.journalPath(state)
	if err != nil {
		return codex.RemoteAuthenticationCleanup{}, NativeInspection{}, err
	}
	raw, err := os.ReadFile(path + ".authentication-stop")
	var j authenticationStopJournal
	if err != nil || b.DecodeStrictJSON(raw, &j, "trailing authentication stop evidence") != nil || j.Binding != binding || j.Failure != "" || j.Proof == nil || !j.Attempt.Attempted || !j.Attempt.Closed || j.Attempt.Unknown || j.Attempt.Process == nil {
		return codex.RemoteAuthenticationCleanup{}, NativeInspection{}, errors.New("authentication cleanup evidence is incomplete or foreign")
	}
	p := j.Attempt.Process
	if p.Kind != "orca_authentication_revoker" || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == state.Process.Terminal || p.Terminal == "" || p.Incarnation == "" || validateAuthenticationStop(*state.RemoteReadiness, *j.Proof) != nil {
		return codex.RemoteAuthenticationCleanup{}, NativeInspection{}, errors.New("authentication cleanup resource binding changed")
	}
	return *j.Proof, j.Attempt, nil
}

func validateAuthenticationStop(ready codex.RemoteReadiness, proof codex.RemoteAuthenticationCleanup) error {
	a, _ := json.Marshal(ready)
	b, _ := json.Marshal(proof.Readiness)
	if string(a) != string(b) || !proof.Revoked || !proof.BrokerClosed || !proof.SocketCleaned || proof.Mode != "host-gh-repository-broker" && proof.Mode != "github-app-repository-broker" {
		return errors.New("original authentication revocation proof changed or is incomplete")
	}
	return nil
}

// This facility revokes only the original in-memory credential lease. Its
// durable intent/owner prevents replacement after an uncertain create; a lost
// read/close can only reconcile that original auxiliary terminal.
func (l *Lifecycle) revokeAuthentication(ctx context.Context, state State) error {
	return l.authenticationStop(ctx, state, true)
}

func (l *Lifecycle) reconcileAuthenticationStop(ctx context.Context, state State) error {
	return l.authenticationStop(ctx, state, false)
}

func (l *Lifecycle) authenticationStop(ctx context.Context, state State, allowCreate bool) error {
	if state.RemoteReadiness == nil || !state.RemoteReadiness.AuthenticationControl {
		return nil
	}
	release, err := l.inspectionGate(ctx)
	if err != nil {
		return err
	}
	defer release()
	path, binding, err := l.journalPath(state)
	if err != nil {
		return err
	}
	path += ".authentication-stop"
	j := authenticationStopJournal{Binding: binding}
	raw, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if existed && (b.DecodeStrictJSON(raw, &j, "trailing authentication stop checkpoint") != nil || j.Binding != binding || !j.Attempt.Attempted) {
		return errors.New("authentication stop checkpoint changed")
	}
	if j.Failure != "" {
		return errAuthenticationProofUnverified
	}
	save := func() error { body, _ := json.Marshal(j); return l.setup.ProcessHost.WriteExecutionFile(path, body) }
	if j.Proof != nil {
		if err := validateAuthenticationStop(*state.RemoteReadiness, *j.Proof); err != nil {
			return err
		}
	}
	if j.Attempt.Closed {
		if j.Proof == nil || j.Attempt.Unknown || j.Attempt.Process == nil {
			return errors.New("authentication stop closure lacks proof")
		}
		return nil
	}
	if !existed {
		if !allowCreate {
			return errors.New("original authentication stop intent unavailable; no new actor created")
		}
		j.Attempt = NativeInspection{Attempted: true, Unknown: true, At: l.clock()}
		if err = save(); err != nil {
			return err
		}
		query, _ := json.Marshal(codex.RemoteAuthenticationQuery{Workspace: state.RemoteReadiness.Workspace, Session: state.RemoteReadiness.SessionID})
		command := "& '" + strings.ReplaceAll(l.profile.RuntimeExecutable, "'", "''") + "' '--authentication-stop' '" + base64.RawURLEncoding.EncodeToString(query) + "'"
		result, e := l.client.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+state.Process.WorktreeID, "--title", "SteerRunner original authentication revocation", "--command", command, "--json")
		var created struct{ Terminal native.Terminal }
		if e != nil || result.CallUncertain || json.Unmarshal(result.Payload, &created) != nil || result.RuntimeID != state.Process.RuntimeID || created.Terminal.Handle == "" || created.Terminal.Handle == state.Process.Terminal || created.Terminal.IncarnationID == "" || created.Terminal.WorktreeID != state.Process.WorktreeID {
			return errors.New("original authentication revoker creation is unverified; no replacement created")
		}
		j.Attempt.Process = &b.ProcessIdentity{Kind: "orca_authentication_revoker", RuntimeID: result.RuntimeID, Terminal: created.Terminal.Handle, Incarnation: created.Terminal.IncarnationID, WorktreeID: state.Process.WorktreeID}
		j.Attempt.Unknown = false
		if err = save(); err != nil {
			return err
		}
	}
	p := j.Attempt.Process
	if p == nil || j.Attempt.Unknown || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == state.Process.Terminal || p.Terminal == "" || p.Incarnation == "" || p.Kind != "orca_authentication_revoker" {
		return errors.New("original authentication revoker owner unavailable; no replacement created")
	}
	if j.Proof == nil {
		for {
			if _, err = l.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, *p); err != nil {
				return err
			}
			result, e := l.client.MetaCall(ctx, "terminal", "read", "--terminal", p.Terminal, "--limit", "100", "--json")
			if e != nil {
				return e
			}
			var output struct {
				Terminal struct {
					Handle string
					Tail   []string
				}
			}
			if json.Unmarshal(result.Payload, &output) != nil || result.RuntimeID != p.RuntimeID || output.Terminal.Handle != p.Terminal {
				return errors.New("authentication stop read identity changed")
			}
			for _, line := range output.Terminal.Tail {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, codex.RemoteAuthenticationPrefix) {
					body, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteAuthenticationPrefix))
					var proof codex.RemoteAuthenticationCleanup
					if e != nil || json.Unmarshal(body, &proof) != nil || validateAuthenticationStop(*state.RemoteReadiness, proof) != nil {
						j.Failure = "unverified"
						return errors.Join(errAuthenticationProofUnverified, save())
					}
					j.Proof = &proof
					if err = save(); err != nil {
						return err
					}
					break
				}
				if line == "remote authentication stop unverified" {
					j.Failure = "unverified"
					return errors.Join(errAuthenticationProofUnverified, save())
				}
			}
			if j.Proof != nil {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	proof, err := l.client.CloseReadonlyTerminal(closeCtx, *p)
	j.Attempt.Closed = err == nil && proof.Disposition == "confirmed"
	if saveErr := save(); saveErr != nil {
		return errors.Join(err, saveErr)
	}
	if !j.Attempt.Closed {
		return errors.Join(err, errors.New("original authentication revoker closure remains unverified"))
	}
	return nil
}

// A failed credential receipt never keeps a known auxiliary process alive after
// the original worker was stopped. Absence/closure is separate from revocation.
func (l *Lifecycle) closeFailedAuthenticationActor(ctx context.Context, state State) error {
	path, binding, err := l.journalPath(state)
	if err != nil {
		return err
	}
	path += ".authentication-stop"
	raw, err := os.ReadFile(path)
	var j authenticationStopJournal
	if err != nil || b.DecodeStrictJSON(raw, &j, "trailing failed authentication checkpoint") != nil || j.Binding != binding || j.Failure != "unverified" || j.Attempt.Process == nil || j.Attempt.Unknown {
		return errors.New("failed authentication actor owner unverified")
	}
	if j.Attempt.Closed {
		return nil
	}
	p := j.Attempt.Process
	if p.Kind != "orca_authentication_revoker" || p.RuntimeID != state.Process.RuntimeID || p.WorktreeID != state.Process.WorktreeID || p.Terminal == state.Process.Terminal {
		return errors.New("failed authentication actor binding changed")
	}
	proof, err := l.client.CloseReadonlyTerminal(ctx, *p)
	j.Attempt.Closed = err == nil && proof.Disposition == "confirmed"
	body, _ := json.Marshal(j)
	saveErr := l.setup.ProcessHost.WriteExecutionFile(path, body)
	if !j.Attempt.Closed {
		return errors.Join(err, saveErr, errors.New("failed authentication actor closure unverified"))
	}
	return saveErr
}
