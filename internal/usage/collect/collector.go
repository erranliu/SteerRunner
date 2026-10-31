// Package collect observes committed Runs on its own worker. Neither queries nor
// the business writer gate perform native log I/O.
package collect

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/access"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

type Collector struct {
	Store    *storage.Store
	Policy   api.Policy
	Instance string
	Leases   *access.Manager
	// The resident engine classifies frozen records before any new log access.
	// Cleanup of existing leases remains independent of observation eligibility.
	CanObserveRun func(string) bool
}
type checkpoint struct {
	Bytes               int
	Hash, Header, Input string
	Policy, Instance    string
	Profile             string
	Binding             string
	Finalized           bool
	EndedAt             time.Time
	Summary             api.SourceSummary
	Issues              []api.Issue
	Missing             int64
}

func (c *Collector) Run(ctx context.Context) {
	c.Leases = &access.Manager{Store: c.Store, Instance: c.Instance, PolicyDigest: c.Policy.Digest(), Permit: c.checkPermit}
	c.cleanup(ctx)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		c.cleanup(ctx)
		c.Tick(ctx)
		select {
		case <-ctx.Done():
			cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			c.cleanup(cleanup)
			cancel()
			return
		case <-tick.C:
		}
	}
}
func (c *Collector) Tick(ctx context.Context) {
	if c.checkPermit(ctx) != nil {
		return
	}
	if c.Leases == nil {
		c.Leases = &access.Manager{Store: c.Store, Instance: c.Instance, PolicyDigest: c.Policy.Digest(), Permit: c.checkPermit}
	}
	runs, err := c.Store.UsageRuns(ctx)
	if err != nil {
		return
	}
	for _, run := range runs {
		if ctx.Err() != nil {
			return
		}
		c.observe(ctx, run)
	}
}
func (c *Collector) observe(ctx context.Context, r *runstate.Record) {
	if c.CanObserveRun != nil && !c.CanObserveRun(r.ID) {
		return
	}
	prior, _ := c.Store.GetRunUsage(ctx, r.ID, 0)
	s := api.Snapshot{RunID: r.ID, AccessCleanupStatus: "not_required", Issues: []api.Issue{}, Coverage: api.Coverage{SourceSummaries: []api.SourceSummary{}, Finalized: true}}
	inputs := []b.LifecycleSnapshot{}
	for _, a := range r.Attempts {
		inputs = append(inputs, a.Snapshot)
	}
	inputs = append(inputs, r.Snapshot)
	// A generation may occur in both attempts and current snapshot. Prefer current.
	byGeneration := map[string]b.LifecycleSnapshot{}
	for _, v := range inputs {
		key := v.Ref.AttemptID + ":" + v.ResourceGeneration
		byGeneration[key] = v
	}
	keys := []string{}
	for key := range byGeneration {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	s.Coverage.ExpectedSources = api.Number(int64(len(keys)))
	journal := map[string]any{}
	events := []api.Event{}
	for _, key := range keys {
		snap := byGeneration[key]
		source := usage.Digest(r.ID, key)
		cpkey := "source:" + source
		var cp checkpoint
		checkpointErr := c.Store.UsageJournal(ctx, cpkey, &cp)
		checkpointInvalid := checkpointErr != nil && !errors.Is(checkpointErr, sql.ErrNoRows)
		binding, p, state, err := orca.UsageBinding(r.Profile, snap, orca.UsageInitialization{LaunchCommand: r.LaunchCommand, CreatedAt: r.CreatedAt})
		if err == nil && binding.RunID != r.ID {
			err = errors.New("IDENTITY_CHANGED")
		}
		if checkpointInvalid {
			err = errors.New("SOURCE_CHANGED")
		}
		profileHash := cp.Profile
		if snap.ResourceGeneration == r.Snapshot.ResourceGeneration && snap.Ref.AttemptID == r.Snapshot.Ref.AttemptID {
			raw, _ := json.Marshal(r.Profile)
			profileHash = usage.Digest(string(raw))
		}
		inputHash := usage.Digest(orca.UsageInputDigest(snap), profileHash, r.LaunchCommand, r.CreatedAt.UTC().Format(time.RFC3339Nano), parsers.Version)
		final := snap.ResourcesFinalized || r.Status == "completed" || r.Status == "stopped" || r.Status == "failed" || snap.ResourceGeneration != r.Snapshot.ResourceGeneration
		allSettled := len(state.Turns) > 0
		for _, turn := range state.Turns {
			if !turn.Settled && !turn.Rejected {
				allSettled = false
			}
		}
		final = final || allSettled || r.Status == "pr_ready" || r.StopStatus == "stopped"
		pending := pendingUsageInput(snap, state)
		final = final && !pending
		completeHistory := cp.Summary.Status == "verified" && cp.Summary.Finalized && len(cp.Issues) == 0 && cp.Missing == 0
		sameObservation := cp.Policy == c.Policy.Digest() && cp.Instance == c.Instance
		if !checkpointInvalid && cp.Finalized && cp.Input == inputHash && (completeHistory || sameObservation) {
			s.Coverage.SourceSummaries = append(s.Coverage.SourceSummaries, cp.Summary)
			s.Issues = append(s.Issues, cp.Issues...)
			s.Coverage.MissingUsageResponses += cp.Missing
			if cp.Summary.Status == "verified" {
				s.Coverage.VerifiedSources++
			}
			if !cp.Summary.Finalized {
				s.Coverage.Finalized = false
			}
			continue
		}
		if !sameObservation || cp.Input != inputHash {
			cp.EndedAt = time.Time{}
		}
		if !c.Policy.Enabled {
			err = errors.New("COLLECTION_DISABLED")
		}
		summary := api.SourceSummary{SourceRef: source, Status: "unverified", Finalized: false, Watermark: cp.Summary.Watermark}
		issues := []api.Issue{}
		missing := int64(0)
		if err == nil {
			observation, cancel := context.WithTimeout(ctx, 30*time.Second)
			var result parsers.Result
			var cleanup string
			result, cleanup, err = c.read(observation, binding, p, state, source, cp)
			cancel()
			if cleanup == "pending" {
				s.AccessCleanupStatus = "pending"
			} else if (cleanup == "revoked" || cleanup == "active") && s.AccessCleanupStatus == "not_required" {
				s.AccessCleanupStatus = cleanup
			}
			if err == nil {
				for i := range result.Issues {
					result.Issues[i].SourceRef = source
				}
				issues = result.Issues
				if !final && prior.Tokens == nil && len(result.Events) == 0 && result.Missing == 0 && result.HeaderHash != "" {
					waiting := true
					for _, issue := range issues {
						if issue.Code != "USAGE_NOT_RECORDED" && issue.Code != "COVERAGE_INCOMPLETE" {
							waiting = false
						}
					}
					if waiting {
						issues = []api.Issue{{Category: "usage", Code: "SESSION_UNBOUND", Retryable: true, SourceRef: source}}
					}
				}
				missing = result.Missing
				for i := range result.Events {
					result.Events[i].SourceRef = source
				}
				events = append(events, result.Events...)
				if result.HeaderHash != "" {
					summary.Status = "verified"
					s.Coverage.VerifiedSources++
					summary.Watermark = result.Watermark
				}
				if final && cp.EndedAt.IsZero() {
					cp.EndedAt = time.Now().UTC()
				}
				summary.Finalized = final && result.Complete
				cp.Bytes = result.SourceBytes
				cp.Hash = result.SourceHash
				cp.Header = result.HeaderHash
				cp.Binding = result.BindingHash
				cp.Input = inputHash
			}
		}
		if err != nil {
			code := errorCode(err)
			if strings.HasPrefix(code, "ACCESS_") {
				issues = append(issues, api.Issue{Category: "access_cleanup", Code: code, Retryable: true, SourceRef: source})
				code = "READ_FAILED"
			}
			issues = append(issues, api.Issue{Category: "usage", Code: code, Retryable: code == "HOST_OFFLINE" || code == "READ_TIMEOUT" || code == "LOG_NOT_FOUND" || code == "SESSION_UNBOUND", SourceRef: source})
			if final {
				if cp.EndedAt.IsZero() {
					cp.EndedAt = time.Now().UTC()
				}
			}
			cp.Input = inputHash
		}
		if pending {
			issues = append(issues, api.Issue{Category: "usage", Code: "COVERAGE_INCOMPLETE", Retryable: true, SourceRef: source})
		}
		if final && !summary.Finalized && !cp.EndedAt.IsZero() && time.Since(cp.EndedAt) >= 30*time.Second {
			issues = append(issues, api.Issue{Category: "usage", Code: "COVERAGE_INCOMPLETE", Retryable: true, SourceRef: source})
		}
		if s.AccessCleanupStatus == "pending" {
			s.Issues = append(s.Issues, api.Issue{Category: "access_cleanup", Code: "ACCESS_CLEANUP_PENDING", Retryable: true, SourceRef: source})
		}
		if !summary.Finalized {
			s.Coverage.Finalized = false
		}
		s.Coverage.SourceSummaries = append(s.Coverage.SourceSummaries, summary)
		s.Issues = append(s.Issues, issues...)
		s.Coverage.MissingUsageResponses += missing
		cp.Summary = summary
		cp.Issues = issues
		cp.Missing = missing
		// Exhausting this bounded observation is not proof of complete source
		// coverage. Only complete history survives policy/instance replacement
		// without observation; gaps may be retried under the new authorization.
		cp.Finalized = summary.Finalized || final && !cp.EndedAt.IsZero() && time.Since(cp.EndedAt) >= 30*time.Second
		cp.Policy = c.Policy.Digest()
		cp.Profile = profileHash
		cp.Instance = c.Instance
		if !checkpointInvalid {
			journal[cpkey] = cp
		}
	}
	// Cleanup facts are independent of finalized usage and can change on recovery.
	if s.AccessCleanupStatus != "not_required" || prior.AccessCleanupStatus != "" && prior.AccessCleanupStatus != "not_required" {
		s.AccessCleanupStatus = c.cleanupStatus(ctx, r.ID, prior.AccessCleanupStatus)
	}
	if s.AccessCleanupStatus != "pending" {
		filtered := []api.Issue{}
		for _, issue := range s.Issues {
			if issue.Category != "access_cleanup" {
				filtered = append(filtered, issue)
			}
		}
		s.Issues = filtered
	}
	if s.AccessCleanupStatus == "active" {
		s.Issues = append(s.Issues, api.Issue{Category: "access_cleanup", Code: "ACCESS_STILL_SHARED"})
	}
	if s.AccessCleanupStatus == "pending" {
		s.Issues = append(s.Issues, api.Issue{Category: "access_cleanup", Code: "ACCESS_CLEANUP_PENDING", Retryable: true})
	}
	_, _ = c.Store.CommitObservedUsage(ctx, s, events, r.Revision, journal, api.CollectionPermit{Instance: c.Instance, PolicyDigest: c.Policy.Digest(), Active: true})
}

func (c *Collector) checkPermit(ctx context.Context) error {
	return c.Store.CheckUsagePermit(ctx, c.Instance, c.Policy.Digest())
}

func pendingUsageInput(s b.LifecycleSnapshot, state orca.State) bool {
	settled := map[string]bool{}
	for _, turn := range state.Turns {
		if turn.Settled || turn.Rejected {
			settled[turn.Command] = true
		} else {
			return true
		}
	}
	return s.PendingTurn != nil && !settled[s.PendingTurn.CommandID] || s.Session.PendingCommandID != "" && !settled[s.Session.PendingCommandID]
}
func errorCode(err error) string {
	var rpc *native.StateRPCError
	if errors.As(err, &rpc) {
		switch rpc.Code {
		case "permission_denied", "access_denied":
			return "LOG_ACCESS_DENIED"
		case "selector_not_found", "file_not_found":
			return "LOG_NOT_FOUND"
		}
	}
	allowed := []string{"COLLECTION_DISABLED", "PRODUCTION_BINDING_UNSUPPORTED", "FORMAT_UNSUPPORTED", "SESSION_UNBOUND", "SESSION_AMBIGUOUS", "IDENTITY_UNVERIFIED", "IDENTITY_CHANGED", "REMOTE_LOG_ACCESS_NOT_CONFIGURED", "LOG_NOT_FOUND", "LOG_ACCESS_DENIED", "HOST_OFFLINE", "READ_TIMEOUT", "READ_FAILED", "SOURCE_CHANGED", "SOURCE_PATH_UNVERIFIED", "BUDGET_EXCEEDED", "ACCESS_CLEANUP_PENDING", "ACCESS_CREATE_OUTCOME_UNKNOWN"}
	for _, code := range allowed {
		if err.Error() == code {
			return code
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "READ_TIMEOUT"
	}
	if os.IsNotExist(err) {
		return "LOG_NOT_FOUND"
	}
	if os.IsPermission(err) {
		return "LOG_ACCESS_DENIED"
	}
	return "READ_FAILED"
}
func localRead(file string) ([]byte, error) {
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Clean(real), filepath.Clean(file)) {
		return nil, errors.New("SOURCE_PATH_UNVERIFIED")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > parsers.MaxSource {
		return nil, errors.New("BUDGET_EXCEEDED")
	}
	raw, err := io.ReadAll(io.LimitReader(f, info.Size()))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != info.Size() {
		return nil, errors.New("SOURCE_CHANGED")
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, errors.New("SOURCE_CHANGED")
	}
	return raw, nil
}
func (c *Collector) read(ctx context.Context, binding parsers.Binding, p orca.Profile, state orca.State, source string, cp checkpoint) (parsers.Result, string, error) {
	var zero parsers.Result
	cleanup := "not_required"
	file, namespace := "", ""
	if p.Environment == "" && p.Terminal != nil {
		host, account, identityErr := localUsageIdentity()
		if identityErr != nil {
			return zero, cleanup, errors.New("IDENTITY_UNVERIFIED")
		}
		binding.HostID = host
		namespace = account
		binding.Namespace = namespace
		if err := checkSourceBinding(binding, cp); err != nil {
			return zero, cleanup, err
		}
		root := filepath.Join(p.Terminal.StateDirectory, "sessions")
		if binding.Agent == "dsh" {
			root = filepath.Join(p.Terminal.StateDirectory, "home", ".dsh", "sessions")
		}
		var files []string
		if binding.Agent == "pi" {
			files, _ = filepath.Glob(filepath.Join(root, "*"+binding.SessionID+".jsonl"))
		} else {
			files, _ = filepath.Glob(filepath.Join(root, "*", binding.SessionID, "session.v4.jsonl.zstd"))
		}
		if len(files) == 0 {
			return zero, cleanup, errors.New("LOG_NOT_FOUND")
		}
		if len(files) != 1 {
			return zero, cleanup, errors.New("SESSION_AMBIGUOUS")
		}
		file = files[0]
	} else {
		host := "local"
		var err error
		if p.Environment != "" {
			host, err = native.UsageHostID(p.Environment)
			if err != nil {
				return zero, cleanup, errors.New("IDENTITY_UNVERIFIED")
			}
			if c.Policy.RemoteLogAccess.Mode != "temporary" {
				return zero, cleanup, errors.New("REMOTE_LOG_ACCESS_NOT_CONFIGURED")
			}
		}
		runtime, err := currentRuntime(ctx, p)
		if err != nil {
			return zero, cleanup, errors.New("HOST_OFFLINE")
		}
		channel, err := native.OpenUsageFiles(ctx, p.Environment, runtime, host)
		if err != nil {
			return zero, cleanup, errors.New("HOST_OFFLINE")
		}
		defer channel.Close()
		sessions, err := channel.Sessions(ctx)
		if err != nil {
			return zero, cleanup, err
		}
		matches := []native.UsageSession{}
		for _, s := range sessions {
			if s.Agent == binding.Agent && s.SessionID == binding.SessionID && parsers.NormalizePath(s.CWD) == parsers.NormalizePath(binding.CWD) {
				matches = append(matches, s)
			}
		}
		if len(matches) == 0 {
			return zero, cleanup, errors.New("LOG_NOT_FOUND")
		}
		if len(matches) != 1 {
			return zero, cleanup, errors.New("SESSION_AMBIGUOUS")
		}
		candidate := matches[0]
		file = candidate.FilePath
		root := filepath.Dir(file)
		if !api.SafeRoot(root) || !strings.HasSuffix(strings.ToLower(file), ".jsonl") || !strings.Contains(filepath.Base(file), binding.SessionID) {
			return zero, cleanup, errors.New("SOURCE_PATH_UNVERIFIED")
		}
		if candidate.CodexHome == "" && p.Environment == "" {
			candidate.CodexHome = localCodexHome(file)
		}
		if candidate.CodexHome == "" || !strings.HasPrefix(parsers.NormalizePath(root), parsers.NormalizePath(candidate.CodexHome)+"/sessions/") {
			return zero, cleanup, errors.New("SOURCE_PATH_UNVERIFIED")
		}
		namespace = codexNamespace(candidate.CodexHome)
		binding.HostID = host
		if p.Environment == "" {
			localHost, localAccount, identityErr := localUsageIdentity()
			if identityErr != nil {
				return zero, cleanup, errors.New("IDENTITY_UNVERIFIED")
			}
			binding.HostID = localHost
			namespace = usage.Digest(localAccount, namespace)
		}
		binding.Namespace = namespace
		if err = checkSourceBinding(binding, cp); err != nil {
			return zero, cleanup, err
		}
		if p.Environment != "" {
			if !c.Policy.Allows(p.Environment, host, root) {
				return zero, cleanup, errors.New("REMOTE_LOG_ACCESS_NOT_CONFIGURED")
			}
			if err = channel.VerifySource(ctx, root, filepath.Base(file)); err != nil {
				return zero, cleanup, err
			}
			lease, err := c.Leases.Acquire(ctx, channel, access.Lease{RunID: binding.RunID, HostID: host, Environment: p.Environment, Runtime: runtime, Root: root, File: filepath.Base(file), Executable: p.Executable, Args: p.Args}, source)
			cleanup = "pending"
			if err != nil {
				if lease.ID != "" {
					clean, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					_, _ = c.Leases.Release(clean, channel, lease, source, func(ctx context.Context, l access.Lease) bool { return safeFolder(ctx, p, l) })
					cancel()
				}
				return zero, cleanup, err
			}
			defer func() {
				clean, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, _ = c.Leases.Release(clean, channel, lease, source, func(ctx context.Context, l access.Lease) bool { return safeFolder(ctx, p, l) })
				status := c.cleanupStatus(clean, binding.RunID, "pending")
				_ = c.Store.SetUsageCleanup(clean, binding.RunID, status)
			}()
			_ = c.Store.SetUsageCleanup(ctx, binding.RunID, "active")
			if err = c.Leases.ValidateRead(ctx, channel, lease); err != nil {
				return zero, cleanup, err
			}
			stat, err := channel.Stat(ctx, lease.FolderID, lease.File)
			if err != nil {
				return zero, cleanup, err
			}
			if stat.IsDirectory || stat.IsSymlink || stat.Size < 0 || stat.Size > parsers.MaxSource {
				return zero, cleanup, errors.New("BUDGET_EXCEEDED")
			}
			raw := make([]byte, 0, int(stat.Size))
			for int64(len(raw)) < stat.Size {
				if err = c.Leases.ValidateRead(ctx, channel, lease); err != nil {
					return zero, cleanup, err
				}
				length := 512 << 10
				if remain := stat.Size - int64(len(raw)); remain < int64(length) {
					length = int(remain)
				}
				chunk, e := channel.ReadChunk(ctx, lease.FolderID, lease.File, int64(len(raw)), length)
				if e != nil {
					return zero, cleanup, e
				}
				if len(chunk) == 0 {
					return zero, cleanup, errors.New("SOURCE_CHANGED")
				}
				raw = append(raw, chunk...)
			}
			binding.Namespace = namespace
			after, e := channel.Stat(ctx, lease.FolderID, lease.File)
			if e != nil {
				return zero, cleanup, e
			}
			if after.Size != stat.Size || after.Mtime == nil || stat.Mtime == nil || *after.Mtime != *stat.Mtime {
				return zero, cleanup, errors.New("SOURCE_CHANGED")
			}
			result, e := parseSnapshot(ctx, binding, raw, cp, false)
			clean, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = c.Leases.Release(clean, channel, lease, source, func(ctx context.Context, l access.Lease) bool { return safeFolder(ctx, p, l) })
			cleanup = c.cleanupStatus(clean, binding.RunID, "pending")
			cancel()
			return result, cleanup, e
		}
	}
	binding.Namespace = namespace
	if err := c.checkPermit(ctx); err != nil {
		return zero, cleanup, err
	}
	raw, err := localRead(file)
	if err != nil {
		return zero, cleanup, err
	}
	returnResult, err := parseSnapshot(ctx, binding, raw, cp, strings.HasSuffix(file, ".zstd"))
	return returnResult, cleanup, err
}

// Local AI Vault entries can omit codexHome. Resolve only an explicit local
// CODEX_HOME, the current user's default home, or an Orca account home rooted
// in that user's application-data directory. An arbitrary dated path is not
// evidence of a trustworthy account namespace. Remote entries never use this.
func localCodexHome(file string) string {
	if !filepath.IsAbs(file) || !api.SafeRoot(filepath.Dir(file)) {
		return ""
	}
	home := filepath.Dir(file)
	for i := 0; i < 4; i++ {
		home = filepath.Dir(home)
	}
	known := []string{os.Getenv("CODEX_HOME")}
	if user, err := os.UserHomeDir(); err == nil {
		known = append(known, filepath.Join(user, ".codex"))
	}
	for _, root := range known {
		if root != "" && filepath.IsAbs(root) && parsers.NormalizePath(root) == parsers.NormalizePath(home) {
			return home
		}
	}
	if config, err := os.UserConfigDir(); err == nil && filepath.Base(home) == "home" && parsers.NormalizePath(filepath.Dir(filepath.Dir(home))) == parsers.NormalizePath(filepath.Join(config, "orca", "codex-accounts")) {
		return home
	}
	return ""
}

func codexNamespace(home string) string {
	normalized := parsers.NormalizePath(home)
	marker := "/codex-accounts/"
	index := strings.LastIndex(normalized, marker)
	if index >= 0 {
		parts := strings.Split(normalized[index+len(marker):], "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] == "home" {
			return usage.Digest("orca-codex-account", parts[0])
		}
	}
	return usage.Digest("codex-profile", normalized)
}
func parseSnapshot(ctx context.Context, b parsers.Binding, raw []byte, cp checkpoint, compressed bool) (parsers.Result, error) {
	if err := checkSourceBinding(b, cp); err != nil {
		return parsers.Result{}, err
	}
	if cp.Bytes > 0 && (len(raw) < cp.Bytes || parsers.TextHash(string(raw[:cp.Bytes])) != cp.Hash) {
		return parsers.Result{}, errors.New("SOURCE_CHANGED")
	}
	r := parsers.Parse(ctx, b, raw, compressed)
	r.BindingHash = sourceBindingHash(b)
	r.SourceBytes = len(raw)
	r.SourceHash = parsers.TextHash(string(raw))
	if cp.Header != "" && r.HeaderHash != "" && cp.Header != r.HeaderHash {
		return parsers.Result{}, errors.New("SOURCE_CHANGED")
	}
	return r, nil
}

func sourceBindingHash(b parsers.Binding) string {
	return usage.Digest(b.HostID, b.Namespace, b.Agent, b.SessionID, parsers.NormalizePath(b.CWD))
}
func checkSourceBinding(b parsers.Binding, cp checkpoint) error {
	if cp.Binding != "" && cp.Binding != sourceBindingHash(b) {
		return errors.New("IDENTITY_CHANGED")
	}
	return nil
}
func currentRuntime(ctx context.Context, p orca.Profile) (string, error) {
	args := append([]string{}, p.Args...)
	args = append(args, "status", "--json")
	if p.Environment != "" {
		args = append(args, "--environment", p.Environment)
	}
	raw, err := native.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, p.Executable, args...).Output()
	})
	if err != nil {
		return "", err
	}
	envelope, err := native.ParseEnvelope(raw)
	if err != nil || envelope.Meta.RuntimeID == "" {
		return "", errors.New("HOST_OFFLINE")
	}
	return envelope.Meta.RuntimeID, nil
}
func safeFolder(ctx context.Context, p orca.Profile, l access.Lease) bool {
	args := append([]string{}, p.Args...)
	// Terminal selectors require the explicit id: prefix even though the file
	// channel accepts the bare folder: scope. A selector rejection proves nothing
	// about live terminals and must never authorize deletion.
	args = append(args, "terminal", "list", "--worktree", "id:folder:"+l.FolderID, "--json")
	if p.Environment != "" {
		args = append(args, "--environment", p.Environment)
	}
	raw, err := native.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, p.Executable, args...).Output()
	})
	if err != nil {
		return false
	}
	envelope, err := native.ParseEnvelope(raw)
	if err != nil || envelope.Meta.RuntimeID != l.Runtime {
		return false
	}
	return emptyTerminalListing(envelope.Result)
}

func emptyTerminalListing(raw json.RawMessage) bool {
	var listing struct {
		Terminals []json.RawMessage `json:"terminals"`
		Total     *int              `json:"totalCount"`
		Truncated *bool             `json:"truncated"`
		HostScope struct {
			Omitted []string `json:"omittedHostIds"`
		} `json:"hostScope"`
	}
	return json.Unmarshal(raw, &listing) == nil && listing.Terminals != nil && len(listing.Terminals) == 0 && listing.Total != nil && *listing.Total == 0 && listing.Truncated != nil && !*listing.Truncated && listing.HostScope.Omitted != nil && len(listing.HostScope.Omitted) == 0
}
func (c *Collector) cleanupStatus(ctx context.Context, run, prior string) string {
	keys, err := c.Store.UsageJournalKeys(ctx, "lease:")
	if err != nil {
		return "pending"
	}
	history, err := c.Store.UsageJournalKeys(ctx, "revocation:")
	if err != nil {
		return "pending"
	}
	latest := map[string]access.Lease{}
	keys = append(keys, history...)
	for _, key := range keys {
		var l access.Lease
		if c.Store.UsageJournal(ctx, key, &l) != nil {
			return "pending"
		}
		if l.RunID != run && !l.Runs[run] {
			continue
		}
		if old, ok := latest[l.ID]; !ok || l.Epoch > old.Epoch {
			latest[l.ID] = l
		}
	}
	if len(latest) == 0 {
		if prior == "revoked" {
			return prior
		}
		return "pending"
	}
	status := "revoked"
	for _, l := range latest {
		if l.CreateUnresolved || l.MutationUnresolved {
			return "pending"
		}
		switch l.Phase {
		case "revoked":
		case "active":
			if len(l.Consumers) == 0 {
				return "pending"
			}
			status = "active"
		default:
			return "pending"
		}
	}
	return status
}
func (c *Collector) cleanup(ctx context.Context) {
	keys, err := c.Store.UsageJournalKeys(ctx, "lease:")
	if err != nil {
		return
	}
	for _, key := range keys {
		var l access.Lease
		if c.Store.UsageJournal(ctx, key, &l) != nil || l.Phase == "revoked" {
			continue
		}
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		profile := orca.Profile{Executable: l.Executable, Args: l.Args, Environment: l.Environment}
		if l.Executable != "" {
			runtime, err := currentRuntime(bounded, profile)
			if err != nil {
				cancel()
				continue
			}
			l.Runtime = runtime
		}
		channel, err := native.OpenUsageFiles(bounded, l.Environment, l.Runtime, l.HostID)
		if err != nil {
			cancel()
			continue
		}
		_, _ = c.Leases.Release(bounded, channel, l, "", func(ctx context.Context, l access.Lease) bool {
			return safeFolder(ctx, orca.Profile{Executable: l.Executable, Args: l.Args, Environment: l.Environment}, l)
		})
		if l.Runs == nil {
			l.Runs = map[string]bool{}
		}
		if l.RunID != "" {
			l.Runs[l.RunID] = true
		}
		for run := range l.Runs {
			_ = c.Store.SetUsageCleanup(bounded, run, c.cleanupStatus(bounded, run, "pending"))
		}
		channel.Close()
		cancel()
	}
}
