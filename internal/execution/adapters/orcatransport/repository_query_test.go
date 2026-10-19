package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const repositoryCacheError = `EPERM: operation not permitted, rename 'C:\Users\worker\AppData\Roaming\orca\orca-environments.json.29180.1792398040614.bcdf4d32.tmp' -> 'C:\Users\worker\AppData\Roaming\orca\orca-environments.json'`

func TestRepositoryCacheConflictClassification(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		retry         bool
	}{
		{"observed", repositoryCacheError, true},
		{"wrapped-stderr", "orca.exe failed: exit status 1 (" + repositoryCacheError + ")\ninvalid JSON", true},
		{"access", strings.Replace(repositoryCacheError, "EPERM", "EACCES", 1), true},
		{"busy", strings.Replace(repositoryCacheError, "EPERM", "EBUSY", 1), true},
		{"forward-slashes", strings.ReplaceAll(repositoryCacheError, `\`, "/"), true},
		{"permission", "EPERM: operation not permitted", false},
		{"open-cache", `EPERM: operation not permitted, open 'C:\Users\worker\AppData\Roaming\orca\orca-environments.json'`, false},
		{"other-file", strings.ReplaceAll(repositoryCacheError, "orca-environments.json", "settings.json"), false},
		{"other-directory", strings.ReplaceAll(repositoryCacheError, `\orca\`, `\other\`), false},
		{"foreign-temp", strings.Replace(repositoryCacheError, "orca-environments.json.29180", "other.29180", 1), false},
		{"nested-temp", strings.Replace(repositoryCacheError, ".29180", `.dir\29180`, 1), false},
		{"not-temporary", strings.Replace(repositoryCacheError, ".tmp'", ".bak'", 1), false},
		{"missing", strings.Replace(repositoryCacheError, "EPERM", "ENOENT", 1), false},
		{"authentication", "unauthorized: authentication rejected", false},
		{"identity", "original Orca runtime identity changed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRepositoryCacheConflict(errors.New(tc.message)); got != tc.retry {
				t.Fatalf("retry = %t, want %t: %s", got, tc.retry, tc.message)
			}
		})
	}
	if isRepositoryCacheConflict(nil) {
		t.Fatal("nil error is retryable")
	}
}

func TestRepositoryQueryRetryBudget(t *testing.T) {
	for _, scenario := range []string{"attempt-limit", "permanent-after-conflict", "cancel-backoff", "caller-deadline", "slow-call", "shared-deadline", "already-canceled"} {
		t.Run(scenario, func(t *testing.T) {
			l, _, _ := fixtureOn(t, "desktop")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "caller-deadline" || scenario == "slow-call" {
				var deadlineCancel context.CancelFunc
				ctx, deadlineCancel = context.WithTimeout(ctx, 60*time.Millisecond)
				defer deadlineCancel()
			}
			if scenario == "shared-deadline" {
				var deadlineCancel context.CancelFunc
				ctx, deadlineCancel = context.WithTimeout(ctx, 500*time.Millisecond)
				defer deadlineCancel()
			}
			if scenario == "already-canceled" {
				cancel()
			}
			calls := 0
			var originalDeadline time.Time
			var logs []string
			l.setup.Logf = func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }
			first := fmt.Errorf("first failure: %s", repositoryCacheError)
			last := errors.New("last failure: permission_denied")
			l.client = native.New(native.Config{Output: func(callCtx context.Context, _ time.Duration, args ...string) ([]byte, error) {
				calls++
				if strings.Join(args, " ") != "repo list --json" {
					t.Fatal("retry escaped inventory query", args)
				}
				deadline, ok := callCtx.Deadline()
				if !ok || time.Until(deadline) > repositoryQueryBudget {
					t.Fatal("CLI call has no shared 15s deadline")
				}
				if originalDeadline.IsZero() {
					originalDeadline = deadline
				} else if !deadline.Equal(originalDeadline) {
					t.Fatal("retry reset the shared deadline")
				}
				if scenario == "slow-call" || scenario == "shared-deadline" && calls > 1 {
					<-callCtx.Done()
					return nil, callCtx.Err()
				}
				if scenario == "cancel-backoff" {
					time.AfterFunc(20*time.Millisecond, cancel)
				}
				if scenario == "permanent-after-conflict" && calls > 1 {
					return nil, last
				}
				if calls == repositoryQueryAttempts {
					last = fmt.Errorf("last cache failure: %s", repositoryCacheError)
					return nil, last
				}
				return nil, first
			}})
			started := time.Now()
			_, err := l.queryRepositoryInventory(ctx)
			wantCalls := 1
			switch scenario {
			case "attempt-limit":
				wantCalls = 5
			case "permanent-after-conflict":
				wantCalls = 2
			case "shared-deadline":
				wantCalls = 2
			case "already-canceled":
				wantCalls = 0
			}
			if err == nil || calls != wantCalls {
				t.Fatal("retry count", calls, wantCalls, err)
			}
			switch scenario {
			case "attempt-limit", "permanent-after-conflict":
				if !errors.Is(err, first) || !errors.Is(err, last) || !strings.Contains(err.Error(), "first failure") || !strings.Contains(err.Error(), "last ") {
					t.Fatal("first/last failure lost", err)
				}
				if scenario == "attempt-limit" && time.Since(started) < 3*time.Second {
					t.Fatal("retry did not back off")
				}
				if len(logs) != calls-1 || !strings.Contains(logs[0], "retry=2/5") || !strings.Contains(logs[0], "first_error=") || !strings.Contains(logs[0], "last_error=") || !strings.Contains(logs[0], "run="+l.setup.Snapshot.Session.WorkflowRunID) {
					t.Fatal("retry progress or original Run identity was not logged", logs)
				}
			case "caller-deadline", "slow-call", "shared-deadline":
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
					t.Fatal("caller deadline did not bound calls and backoff", err)
				}
			default:
				if !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
					t.Fatal("cancellation did not interrupt retry", err)
				}
			}
		})
	}
}

func TestRepositoryCacheRetryNeverReplaysMutations(t *testing.T) {
	for _, scenario := range []string{"recovers", "inventory-denied", "inventory-exhausted", "creation-unknown", "send-unknown"} {
		t.Run(scenario, func(t *testing.T) {
			l, f, s := fixtureOn(t, "desktop")
			counts := map[string]int{}
			calls := workspaceCalls(l, func(args []string) ([]byte, bool) {
				command := args[0] + " " + args[1]
				counts[command]++
				if command == "repo list" {
					if scenario == "inventory-denied" {
						return workspaceQueryError("EPERM: permission denied"), true
					}
					if counts[command] == 1 || scenario == "inventory-exhausted" {
						return workspaceQueryError(repositoryCacheError), true
					}
				}
				if scenario == "creation-unknown" && command == "worktree create" || scenario == "send-unknown" && command == "terminal send" {
					return workspaceQueryError(repositoryCacheError), true
				}
				return nil, false
			})
			req := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
			receipt, _, err := l.Execute(context.Background(), req)
			if scenario == "inventory-denied" {
				if err == nil || receipt.Disposition != b.OperationRejected || len(*calls) != 1 {
					t.Fatal("permanent inventory error retried or created resources", receipt, err, *calls)
				}
				return
			}
			if scenario == "inventory-exhausted" {
				if err == nil || receipt.Disposition != b.OperationRejected || counts["repo list"] != 5 || len(*calls) != 5 {
					t.Fatal("exhausted query created resources or exceeded budget", receipt, err, counts)
				}
				return
			}
			if counts["repo list"] != 2 || counts["worktree create"] != 1 || counts["terminal send"] != 0 {
				t.Fatal("inventory retry replayed a mutation", counts)
			}
			if scenario == "creation-unknown" {
				if err == nil || receipt.Disposition != b.OperationUnknown {
					t.Fatal(receipt, err)
				}
				req.Operation.Receipt, _ = json.Marshal(receipt)
				before := len(*calls)
				_, _, _ = l.Reconcile(context.Background(), req)
				if len(*calls) != before {
					t.Fatal("unknown creation without receipt rediscovered/recreated resources")
				}
				return
			}
			if err != nil || receipt.Disposition != b.OperationConfirmed {
				t.Fatal(receipt, err)
			}
			bind(&s, receipt)
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			f.writeReply = true
			send := request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})
			sent, err := l.submit(context.Background(), send)
			if scenario == "send-unknown" {
				if err == nil {
					t.Fatal("send failure disappeared")
				}
				bind(&s, sent)
				send.Snapshot = s
				_, _, _ = l.Reconcile(context.Background(), send)
			} else if err != nil || !sent.State.InputAccepted {
				t.Fatal(sent, err)
			}
			if counts["worktree create"] != 1 || counts["terminal create"] != 0 || counts["terminal send"] != 1 || counts["repo list"] != 2 {
				t.Fatal("original resource/input was replayed", counts)
			}
		})
	}
}

func TestRepositoryQueryScopeRevocationPreventsCreation(t *testing.T) {
	l, _, s := fixtureOn(t, "desktop")
	scopes := 0
	l.setup.ReadOnlyPreparation = func(ctx context.Context, query func(context.Context) error) error {
		scopes++
		if err := query(ctx); err != nil {
			t.Fatal(err)
		}
		return context.Canceled // Stop committed while the inventory was in flight.
	}
	calls := workspaceCalls(l, nil)
	receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if !errors.Is(err, context.Canceled) || receipt.Disposition != b.OperationRejected || scopes != 1 || len(*calls) != 1 || (*calls)[0][0] != "repo" {
		t.Fatal("successful query bypassed revoked creation authority", receipt, err, *calls)
	}
}
