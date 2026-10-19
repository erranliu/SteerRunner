package orcatransport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"path"
	"regexp"
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

const (
	repositoryQueryAttempts = 5
	repositoryQueryBudget   = 15 * time.Second
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

// Retry only this read-only inventory, before any worktree or agent creation.
// Do not move this loop around prepareWorkspace or the general CLI transport:
// failed mutations may already have succeeded and require exact reconciliation.
func (l *Lifecycle) queryRepositoryInventory(ctx context.Context) (native.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, repositoryQueryBudget)
	defer cancel()
	logf := l.setup.Logf
	if logf == nil {
		logf = log.Printf
	}
	var result native.Result
	var first, last error
	attempts := 0
	failed := func(cause error) (native.Result, error) {
		if attempts == 0 {
			return result, fmt.Errorf("Orca repository inventory query not started: %w", cause)
		}
		return result, fmt.Errorf("Orca repository inventory query failed after %d/%d attempts (budget %s): %w",
			attempts, repositoryQueryAttempts, repositoryQueryBudget,
			errors.Join(fmt.Errorf("first error: %w", first), fmt.Errorf("last error: %w", last), cause))
	}
	backoff := 250 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return failed(err)
		}
		attempts++
		var err error
		result, err = l.client.MetaCall(ctx, "repo", "list", "--json")
		if err == nil && ctx.Err() == nil {
			return result, nil
		}
		last = errors.Join(err, ctx.Err())
		if first == nil {
			first = last
		}
		if ctx.Err() != nil || attempts == repositoryQueryAttempts || !isRepositoryCacheConflict(last) {
			return failed(ctx.Err())
		}
		// +/- 10% jitter prevents concurrent Runs from retrying in lockstep.
		delay := backoff - backoff/10 + time.Duration(rand.Int64N(int64(backoff/5)+1))
		if delay > 2*time.Second {
			delay = 2 * time.Second
		}
		logf("Orca repository cache conflict run=%s attempt=%s retry=%d/%d delay=%s budget=%s first_error=%q last_error=%q",
			l.setup.Snapshot.Session.WorkflowRunID, l.setup.Snapshot.Session.AttemptID,
			attempts+1, repositoryQueryAttempts, delay, repositoryQueryBudget, first, last)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return failed(ctx.Err())
		case <-timer.C:
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}
