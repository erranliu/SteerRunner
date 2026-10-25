package orcatransport

import (
	"context"
	"errors"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestPreparationReadRecoveryRetainsBudget(t *testing.T) {
	for _, mode := range []string{"expired", "attempts", "backoff", "clock-rollback"} {
		t.Run(mode, func(t *testing.T) {
			l, _, _ := fixture(t)
			now := time.Now().UTC()
			q := b.PreparationQuery{Visible: false, Wait: b.ExecutionWait{Contract: b.ExecutionWaitContract, OperationID: "original", Query: "runtime_status", StartedAt: now.Add(-time.Minute), UpdatedAt: now, DeadlineAt: now.Add(time.Second), CallsStarted: 3, MaxCalls: 5, Stage: "queue", ReasonCode: "orca_cli_busy"}}
			if mode == "expired" {
				q.Wait.DeadlineAt = now.Add(-time.Second)
			}
			if mode == "attempts" {
				q.Wait.CallsStarted = 5
			}
			if mode == "backoff" {
				next := now.Add(120 * time.Millisecond)
				q.Wait.Stage = "backoff"
				q.Wait.ReasonCode = "orca_query_transient"
				q.Wait.NextAttemptAt = &next
			}
			if mode == "clock-rollback" {
				q.Wait.UpdatedAt = now.Add(time.Minute)
			}
			originalDeadline := q.Wait.DeadlineAt
			ctx := b.WithPreparationProgress(context.Background(), func(_ string, update func(*b.PreparationQuery) error) (*b.PreparationQuery, error) {
				if err := update(&q); err != nil {
					return nil, err
				}
				copy := q
				return &copy, nil
			})
			calls := 0
			l.client = native.New(native.Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
				calls++
				if mode == "backoff" && time.Since(now) < 100*time.Millisecond {
					t.Error("restart skipped original backoff")
				}
				return []byte(`{"ok":true,"result":{},"_meta":{"runtimeId":"original"}}`), nil
			}})
			_, err := l.queryPreparation(ctx, "runtime_status", "status", "--json")
			if mode == "backoff" {
				if err != nil || calls != 1 || q.Wait.CallsStarted != 4 {
					t.Fatal(calls, q, err)
				}
			} else if err == nil || calls != 0 {
				t.Fatal("recovery renewed or dispatched invalid budget", calls, q, err)
			}
			if !q.Wait.DeadlineAt.Equal(originalDeadline) {
				t.Fatal("deadline renewed")
			}
		})
	}
}

func TestPreparationTransientEligibility(t *testing.T) {
	for _, tc := range []struct {
		code string
		err  error
		want bool
	}{
		{"", context.DeadlineExceeded, true}, {"", context.Canceled, false}, {"", &native.CLIQueueError{Cause: context.DeadlineExceeded}, false},
		{"transport_unavailable", errors.New("transport"), true}, {"permission_denied", context.DeadlineExceeded, false},
		{"", errors.New("exit status 1"), false}, {"", errors.New("invalid character in JSON"), false}, {"", errors.New("login required"), false},
	} {
		if got := preparationTransient(native.Result{ErrorCode: tc.code}, tc.err); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestPreparationRevokedBeforeOwnerClaimSettlesWithoutDispatch(t *testing.T) {
	l, f, s := fixture(t)
	launches := 0
	l.setup.LaunchExecutionOwner = func(context.Context, string) error { launches++; return errors.New("unexpected owner launch") }
	r := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
	receipt, _, err := l.Reconcile(b.WithPreparationRecovery(context.Background(), false), r)
	if err != nil || receipt.Disposition != b.OperationRejected || launches != 0 || len(f.calls) != 0 {
		t.Fatal("revoked unclaimed preparation queried or created", receipt, err, launches, f.calls)
	}
}
