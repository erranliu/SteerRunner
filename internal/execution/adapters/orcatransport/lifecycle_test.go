package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os/exec"

	"os"

	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeHost struct {
	terminalHandle                string
	callerFileWriter              func(string, []byte) error
	nativeSends                   int
	lostSubmitReply               bool
	nativePrompt                  string
	nativeReply                   string
	mu                            sync.Mutex
	root, nativePath, incarnation string
	calls                         [][]string
	lines                         []string
	writeReply, unknown, clipped  bool
	draft                         string
	screen                        bool
	unstarted, busy               bool
	commandMode, launched         bool
	delayLaunchIdentity           bool
	collapsedEcho                 bool
	waitFailures                  int
	waitBlocked                   string
	rejectInput                   bool
	rejectWrongRequest            bool
	rejectUnavailable             bool
	waitCalls                     int
	waitBudgets                   []time.Duration
	workspace                     map[string]any
	repoInventory                 any
	failShow, unknownClose        bool
}

func (f *fakeHost) InspectProcess(*b.ProcessIdentity) string { return "unknown" }

func (f *fakeHost) WriteExecutionFile(path string, data []byte) error {
	if f.callerFileWriter != nil {
		return f.callerFileWriter(path, data)
	}
	// Runner-owned local journals are independent of a terminal's workspace
	// file channel (which remains unavailable in this protocol fixture).
	if filepath.Base(filepath.Dir(path)) == "orca-initial-terminals" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		return os.WriteFile(path, data, 0600)
	}
	return errors.New("terminal path has no file channel")
}
func (f *fakeHost) OutputCommand(_ context.Context, c b.LaunchCommand) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), c.Args...))
	arg := func(key string) string {
		for i, v := range c.Args {
			if v == key && i+1 < len(c.Args) {
				return c.Args[i+1]
			}
		}
		return ""
	}
	if len(c.Args) < 2 {
		return nil, errors.New("unexpected primitive")
	}
	path := f.root
	if f.nativePath != "" {
		path = f.nativePath
	}
	handle := f.terminalHandle
	if handle == "" {
		handle = "terminal"
	}
	terminal := map[string]any{"handle": handle, "incarnationId": f.incarnation, "worktreeId": "repo::" + path, "worktreePath": path, "branch": "worker", "connected": true, "agentIdentity": "codex", "status": "running"}
	if f.commandMode && (!f.launched || f.delayLaunchIdentity) {
		terminal["agentIdentity"] = ""
	}
	var result any
	switch c.Args[0] + " " + c.Args[1] {
	case "status --json":
		result = map[string]any{"runtime": map[string]any{"state": "ready", "capabilities": []string{"terminal.prompt-delivery.v1"}}}
	case "repo list":
		result = f.repoInventory
		if result == nil {
			result = map[string]any{"repos": []any{map[string]any{"id": "repo", "kind": "git", "path": "/srv/" + repositoryDirectoryName(f.root)}}}
		}
	case "worktree create":
		f.workspace = map[string]any{"id": "repo::" + path, "instanceId": "original-workspace", "repoId": "repo", "path": path, "branch": "worker", "head": arg("--base-branch"), "baseRef": arg("--base-branch"), "displayName": arg("--name")}
		result = map[string]any{"worktree": f.workspace, "agentTerminalHandle": "terminal"}
	case "worktree show":
		result = map[string]any{"worktree": f.workspace}
	case "terminal show":
		if f.failShow {
			return nil, errors.New("offline before close")
		}
		result = map[string]any{"terminal": terminal}
	case "terminal list":
		result = map[string]any{"terminals": []any{terminal}}
	case "terminal wait":
		f.waitCalls++
		f.waitBudgets = append(f.waitBudgets, c.Timeout)
		if f.waitFailures > 0 {
			f.waitFailures--
			return json.Marshal(map[string]any{"ok": false, "error": map[string]any{"code": "timeout", "message": "timeout"}, "_meta": map[string]any{"runtimeId": "runtime"}})
		}
		result = map[string]any{"wait": map[string]any{"satisfied": !f.busy && f.waitBlocked == "", "blockedReason": f.waitBlocked}}
	case "terminal read":
		if f.screen || strings.Contains(strings.Join(c.Args, " "), "--screen") {
			result = map[string]any{"terminal": map[string]any{"handle": handle, "source": "screen", "tail": f.lines, "oldestCursor": "0", "nextCursor": "0", "latestCursor": "0", "truncated": true, "limited": f.clipped, "draft": f.draft}}
			break
		}
		cursor := 0
		if arg("--cursor") != "" {
			cursor, _ = strconv.Atoi(arg("--cursor"))
		}
		if cursor > len(f.lines) {
			return nil, errors.New("cursor reset")
		}
		result = map[string]any{"terminal": map[string]any{"handle": handle, "source": "stream", "tail": f.lines[cursor:], "oldestCursor": "0", "nextCursor": strconv.Itoa(len(f.lines)), "latestCursor": strconv.Itoa(len(f.lines)), "truncated": f.clipped, "limited": false}}
	case "terminal send":
		if f.rejectUnavailable {
			return json.Marshal(map[string]any{"ok": false, "error": map[string]any{"code": "runtime_unavailable", "message": "runtime unavailable"}, "_meta": map[string]any{"runtimeId": "runtime"}})
		}
		if f.rejectInput {
			id := arg("--retry-request")
			if f.rejectWrongRequest {
				id = "foreign"
			}
			result = map[string]any{"send": map[string]any{"handle": handle, "accepted": false, "prompt": map[string]any{"requestId": id, "processIncarnation": f.incarnation}}}
			break
		}
		if f.unknown {
			return nil, errors.New("lost response")
		}
		text := arg("--text")

		if f.commandMode && !f.launched {
			f.launched = true
			result = map[string]any{"send": map[string]any{"handle": handle, "accepted": true, "prompt": map[string]any{"requestId": arg("--retry-request"), "processIncarnation": f.incarnation}}}
			break
		}
		if strings.Contains(text, "SL_RESULT") || strings.Contains(text, "reply_b64") || strings.Contains(text, "nonce=") {
			return nil, errors.New("transport changed the worker prompt")
		}
		if f.collapsedEcho {
			f.lines = append(f.lines, "› Implement the bound Spec…", "     1 -previous tool output", "    + Show details")
		} else {
			f.lines = append(f.lines, "› "+text)
		}
		f.nativeSends++
		f.nativePrompt = text
		if f.writeReply {
			f.nativeReply = `{"summary":"original result"}`
		}
		if f.writeReply {
			f.lines = append(f.lines, `• {"summary":"original result"}`, "  Worked for "+strconv.Itoa(len(f.lines))+"s • 18:50", "› Ask Codex to do anything")
		}

		stages := []string{"input_accepted", "turn_started"}
		if f.unstarted {
			stages = stages[:1]
		}
		if f.lostSubmitReply {
			return nil, errors.New("prompt applied; response lost")
		}
		result = map[string]any{"send": map[string]any{"handle": handle, "accepted": true, "prompt": map[string]any{"requestId": arg("--retry-request"), "processIncarnation": f.incarnation, "stages": stages, "provider": "codex", "observation": "supported"}}}
	case "terminal close":
		if f.unknownClose {
			return nil, errors.New("close response lost")
		}
		result = map[string]any{"close": map[string]any{"handle": handle, "ptyKilled": true}}
	default:
		return nil, errors.New("unexpected resource or orchestration dependency")
	}
	return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": "runtime"}})
}

func fixture(t *testing.T) (*Lifecycle, *fakeHost, b.LifecycleSnapshot) { return fixtureOn(t, "") }
func fixtureOn(t *testing.T, environment string) (*Lifecycle, *fakeHost, b.LifecycleSnapshot) {
	t.Helper()
	f := &fakeHost{root: t.TempDir(), incarnation: "original-incarnation"}
	if environment != "" {
		f.nativePath = "/srv/owned-worker"
	}
	p := Profile{Executable: "orca.exe", WaitTimeout: "1s", Environment: environment}
	raw, _ := json.Marshal(p)
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: p.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: strings.Repeat("d", 32), AttemptID: strings.Repeat("b", 32)}, Session: b.SessionContext{ProjectID: "project", ExecutionID: strings.Repeat("a", 32), AttemptID: strings.Repeat("b", 32), WorkflowRunID: strings.Repeat("d", 32), CorrelationID: "1", NamePrefix: "sr"}, ConfigurationDigest: "config", ResourceGeneration: "generation", WorkspacePlan: b.WorkspaceIntent{BaseCommit: strings.Repeat("e", 40)}}
	version := ProfileVersion
	if environment != "" {
		version = NativeRemoteProfileVersion
	}
	data, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: version, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: "project", ProfileID: "profile", AcceptedConfigVersion: "accepted"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	port, err := frozenFixtureLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{data}, ProfileID: "profile", AcceptedConfigVersion: "accepted", ProcessHost: f, RepositoryPath: f.root, RunDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return port, f, s
}
func request(s b.LifecycleSnapshot, action b.LifecycleAction, input b.LifecycleInput) b.LifecycleRequest {
	ref := s.Ref
	return b.LifecycleRequest{Snapshot: s, Input: input, Operation: b.Operation{Version: 1, ID: "original-operation-" + string(action), Action: string(action), PayloadDigest: "digest", Ref: &ref, LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, ConfigurationDigest: s.ConfigurationDigest, ResourceGeneration: s.ResourceGeneration, InputBindingDigest: b.LifecycleBindingDigest(s), Ref: ref}}}
}
func bind(s *b.LifecycleSnapshot, r b.LifecycleReceipt) {
	if r.Workspace != nil {
		s.Workspace = b.WorkspaceIntent{Path: r.Workspace.Worktree, Branch: r.Workspace.Branch, BaseCommit: r.Workspace.Base}
		s.Ref.WorkspacePath = s.Workspace.Path
		s.Ref.WorkspaceBranch = s.Workspace.Branch
		s.Ref.BaseCommit = s.Workspace.BaseCommit
	}
	if r.Target != nil {
		s.Ref.Target = *r.Target
	}
	if r.Process != nil {
		s.Ref.External = *r.Process
		s.Session.SessionID = r.State.SessionID
		if r.State.SessionGeneration != "" {
			s.Session.Generation = r.State.SessionGeneration
		}
	}
	if len(r.Implementation) > 0 {
		s.Implementation = r.Implementation
	}
}

func (f *fakeHost) StartManaged(*exec.Cmd) (b.StopHandle, error) {
	return nil, errors.New("unexpected process start")
}

func (f *fakeHost) NewProcessGeneration() string { return "generation" }
