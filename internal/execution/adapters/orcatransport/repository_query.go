package orcatransport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"regexp"
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const (
	repositoryQueryAttempts = 5
	repositoryQueryBudget   = 90 * time.Second
)

var cacheRenameError = regexp.MustCompile(`\b(?:EPERM|EACCES|EBUSY):[^\r\n]*?, rename '([^'\r\n]+)' -> '([^'\r\n]+)'`)

// Only the known Orca cache replacement qualifies, never a generic permission
// error. The temporary file must be a sibling of the exact destination file.
func isRepositoryCacheConflict(err error) bool {
	if err == nil {
		return false
	}
	for _, match := range cacheRenameError.FindAllStringSubmatch(err.Error(), -1) {
		from := strings.ToLower(strings.ReplaceAll(match[1], `\`, "/"))
		to := strings.ToLower(strings.ReplaceAll(match[2], `\`, "/"))
		if strings.HasSuffix(to, "/orca/orca-environments.json") &&
			path.Dir(from) == path.Dir(to) && strings.HasPrefix(from, to+".") && strings.HasSuffix(from, ".tmp") {
			return true
		}
	}
	return false
}

func preparationTransient(result native.Result, err error) bool {
	if err == nil || native.IsCLIQueueError(err) || errors.Is(err, context.Canceled) {
		return false
	}
	if isRepositoryCacheConflict(err) {
		return true
	}
	switch result.ErrorCode {
	case "transport_unavailable", "connection_closed", "connection_refused", "timeout", "deadline_exceeded":
		return true
	case "":
	default:
		return false
	}
	var network net.Error
	var backend *b.BackendError
	return errors.Is(err, context.DeadlineExceeded) || errors.As(err, &network) && (network.Timeout() || network.Temporary()) || errors.As(err, &backend) && backend.Code == b.BackendUnreachable
}

func (l *Lifecycle) queryRepositoryInventory(ctx context.Context) (native.Result, error) {
	return l.queryPreparation(ctx, "repository_inventory", "repo", "list", "--json")
}

func (l *Lifecycle) queryPreparation(ctx context.Context, name string, args ...string) (result native.Result, outErr error) {
	query := func(ctx context.Context) error {
		var local b.PreparationQuery
		progress := b.PreparationProgressFrom(ctx)
		if progress == nil {
			progress = func(_ string, update func(*b.PreparationQuery) error) (*b.PreparationQuery, error) {
				if err := update(&local); err != nil {
					return nil, err
				}
				copy := local
				return &copy, nil
			}
		}
		q, err := progress(name, func(q *b.PreparationQuery) error {
			if q.Wait.StartedAt.IsZero() {
				now := time.Now().UTC()
				deadline := now.Add(repositoryQueryBudget)
				if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
					deadline = parent.UTC()
				}
				q.Wait = b.ExecutionWait{Contract: b.ExecutionWaitContract, Query: name, StartedAt: now, UpdatedAt: now, DeadlineAt: deadline, MaxCalls: repositoryQueryAttempts, Stage: "queue", ReasonCode: "orca_cli_busy", Diagnostic: "Orca CLI channel is busy."}
				q.Visible = true
			}
			if parent, ok := ctx.Deadline(); ok && parent.Before(q.Wait.DeadlineAt) {
				q.Wait.DeadlineAt = parent.UTC()
			}
			return nil
		})
		if err != nil {
			return err
		}
		if time.Now().UTC().Before(q.Wait.UpdatedAt.Add(-time.Second)) {
			return errors.New("preparation query clock moved backwards; automatic dispatch stopped")
		}
		// Convert recovered UTC time into a monotonic remaining budget.
		ctx, cancel := context.WithTimeout(ctx, time.Until(q.Wait.DeadlineAt))
		defer cancel()
		update := func(stage, reason, diagnostic string, next *time.Time, called bool) error {
			q, err = progress(name, func(q *b.PreparationQuery) error {
				q.Visible = true
				q.Wait.Stage, q.Wait.ReasonCode, q.Wait.Diagnostic, q.Wait.NextAttemptAt = stage, reason, diagnostic, next
				if called {
					q.Wait.CallsStarted++
				}
				return nil
			})
			return err
		}
		exhausted := func(reason string, cause error) error {
			if err := update("exhausted", reason, "The original Orca preparation query budget is exhausted.", nil, false); err != nil {
				return errors.Join(cause, err)
			}
			return errors.Join(cause, errors.New(reason))
		}
		var first, last error
		for {
			if ctx.Err() != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return exhausted("wait_budget_exhausted", errors.Join(first, last, ctx.Err()))
				}
				return ctx.Err()
			}
			if q.Wait.CallsStarted >= repositoryQueryAttempts {
				return exhausted("query_attempts_exhausted", errors.Join(first, last))
			}
			if q.Wait.NextAttemptAt != nil {
				if err := waitPreparation(ctx, time.Until(*q.Wait.NextAttemptAt)); err != nil {
					continue
				}
			}
			if err := update("queue", "orca_cli_busy", "Orca CLI channel is busy.", nil, false); err != nil {
				return err
			}
			result, err = l.client.PreparationCall(ctx, l.nativeAccess, func() error {
				if err := ctx.Err(); err != nil {
					return err
				}
				return update("calling", "orca_query_running", "Orca preparation query is running.", nil, true)
			}, args...)
			if err == nil && ctx.Err() == nil {
				return nil
			}
			if ctx.Err() != nil {
				continue
			}
			if native.IsCLIQueueError(err) {
				return err
			}
			last = err
			if first == nil {
				first = err
			}
			if !preparationTransient(result, err) {
				return errors.Join(first, last)
			}
			if q.Wait.CallsStarted >= repositoryQueryAttempts {
				return exhausted("query_attempts_exhausted", errors.Join(first, last))
			}
			delay := time.Second * time.Duration(1<<(q.Wait.CallsStarted-1))
			next := time.Now().UTC().Add(delay)
			if l.setup.Logf != nil {
				l.setup.Logf("Orca preparation read run=%s query=%s retry=%d/5 delay=%s first_error=%q last_error=%q", l.setup.Snapshot.Session.WorkflowRunID, name, q.Wait.CallsStarted+1, delay, first, last)
			}
			if err := update("backoff", "orca_query_transient", fmt.Sprint(last), &next, false); err != nil {
				return err
			}
		}
	}
	if l.setup.ReadOnlyPreparation != nil {
		outErr = l.setup.ReadOnlyPreparation(ctx, query)
	} else {
		outErr = query(ctx)
	}
	return result, outErr
}

func waitPreparation(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
