package orcatransport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaPreSendReconnect(t *testing.T) {
	for _, scenario := range []string{"open", "repeated-open", "query", "identity", "authentication", "provider-changed", "terminal-changed", "exhausted", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			l, host, s := fixture(t)
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			l.profile.WaitTimeout = "3s"
			base := fixtureState(10, "previous", "done")
			first := &countedStateFeed{controlledStateFeed: &controlledStateFeed{initial: base, current: base}}
			restored := &countedStateFeed{controlledStateFeed: &controlledStateFeed{initial: base, current: base}}
			transient := &native.StateConnectionError{Cause: errors.New("dial failed")}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "exhausted" {
				l.profile.WaitTimeout = "20ms"
			}
			opens := 0
			l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
				opens++
				if scenario == "repeated-open" && opens < 3 {
					return nil, transient
				}
				if opens > 1 {
					return restored, nil
				}
				switch scenario {
				case "identity":
					return nil, native.ErrStateIdentityChanged
				case "authentication":
					return nil, errors.New("pairing authentication rejected")
				case "cancelled":
					cancel()
					return nil, transient
				case "query":
					first.err = transient
					return first, nil
				case "provider-changed":
					restored.initial.ProviderSession.ID = "foreign"
					restored.current = restored.initial
				case "terminal-changed":
					host.incarnation = "foreign"
				}
				return nil, transient
			}
			r, err = l.submit(ctx, request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			defer l.closeStateFeed()
			if scenario == "open" || scenario == "query" || scenario == "repeated-open" {
				wantOpens := 2
				if scenario == "repeated-open" {
					wantOpens = 3
				}
				if err != nil || r.Disposition != b.OperationAccepted || host.nativeSends != 1 || opens != wantOpens {
					t.Fatalf("recovery did not send exactly once: %+v, %v, opens=%d sends=%d", r, err, opens, host.nativeSends)
				}
				if scenario == "query" && first.closes != 1 {
					t.Fatal("failed subscription was retained")
				}
				// Re-execution of an attempted send remains reconciliation-only.
				bind(&s, r)
				if _, err = l.submit(ctx, request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})); err == nil || host.nativeSends != 1 {
					t.Fatal("attempted input replayed")
				}
			} else {
				if err == nil || r.Disposition != b.OperationRejected || host.nativeSends != 0 {
					t.Fatalf("unverified readiness sent input: %+v, %v", r, err)
				}
				if (scenario == "identity" || scenario == "authentication" || scenario == "cancelled" || scenario == "exhausted") && opens != 1 {
					t.Fatal("terminal failure retried", opens)
				}
			}
		})
	}
}

func TestOrcaPrepareSessionReconnect(t *testing.T) {
	l, host, s := fixture(t)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	base := fixtureState(10, "previous", "done")
	opens := 0
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
		opens++
		if opens == 1 {
			return nil, &native.StateConnectionError{Cause: context.DeadlineExceeded}
		}
		return &controlledStateFeed{initial: base, current: base}, nil
	}
	s, _ = prepared(t, l, s, strings.Repeat("c", 32))
	state, err := l.state(s)
	if err != nil || opens != 2 || len(state.Turns) != 1 || state.Turns[0].Attempted || host.nativeSends != 0 {
		t.Fatal("preparation recovery lost baseline or sent input", err)
	}
}

func TestOrcaPreSendReconnectCancellationDuringBackoff(t *testing.T) {
	l, _, s := fixture(t)
	opens := 0
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
		opens++
		return nil, &native.StateConnectionError{Cause: context.DeadlineExceeded}
	}
	// Use a valid original terminal so the failure occurs in connection setup.
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = l.ready(ctx, s)
	if !errors.Is(err, context.DeadlineExceeded) || opens != 1 {
		t.Fatal("cancellation lost", err)
	}
}
