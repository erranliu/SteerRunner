//go:build windows && live

package native

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

type stabilityRun struct {
	t                      *testing.T
	m                      livetest.Manifest
	r                      livetest.StabilityResult
	dir                    string
	instance               contracts.ServiceInfo
	original               contracts.ExecutionView
	process                b.ProcessIdentity
	provider               string
	pane                   string
	blocked                bool
	seq                    int
	started                time.Time
	cursor                 string
	serviceStartAttempted  bool
	workingAt              int64
	observedPrompt         string
	disconnectedAt         time.Time
	auxiliary              map[string]b.ProcessIdentity
	auxiliaryCreatePending bool
	auditedPrompts         []string
}

func (s *stabilityRun) note(kind string, value any) {
	s.seq++
	body, err := json.Marshal(map[string]any{"seq": s.seq, "at": time.Now().UTC(), "kind": kind, "value": value})
	if err == nil {
		var f *os.File
		f, err = os.OpenFile(filepath.Join(*evidenceFlag, "faults.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			_, err = f.Write(append(body, '\n'))
			_ = f.Close()
		}
	}
	if err != nil {
		s.t.Fatal(err)
	}
}
func (s *stabilityRun) stage(value string) {
	s.r.Stage = value
	save(s.t, "scenario.json", s.r)
	s.note("stage", value)
	if s.r.RunID != "" {
		args := []string{"events", "--run-id", s.r.RunID, "--limit", "200"}
		if s.cursor != "" {
			args = append(args, "--after", s.cursor)
		}
		response, err := command(s.m, s.dir, args, nil)
		if err == nil {
			s.note("events", response.Data)
			raw, _ := json.Marshal(response.Data)
			var data contracts.EventPage
			_ = json.Unmarshal(raw, &data)
			if data.NextCursor != "" {
				s.cursor = data.NextCursor
			}
		}
	}
}
func (s *stabilityRun) check(name string, ok bool, evidence string) {
	for i := range s.r.Assertions {
		if s.r.Assertions[i].Name == name {
			s.r.Assertions[i].Passed = s.r.Assertions[i].Passed && ok
			s.note("assertion-recheck", map[string]any{"name": name, "passed": ok, "evidence": evidence})
			if !ok {
				s.t.Fatalf("stability assertion %s failed: %s", name, evidence)
			}
			return
		}
	}
	s.r.Assertions = append(s.r.Assertions, livetest.Assertion{Name: name, Passed: ok, Evidence: evidence})
	save(s.t, "scenario.json", s.r)
	if !ok {
		s.t.Fatalf("stability assertion %s failed: %s", name, evidence)
	}
}
func (s *stabilityRun) view() contracts.RunView {
	v, err := inspect(s.m, s.dir, s.r.RunID, false)
	if err != nil {
		s.t.Fatal(err)
	}
	save(s.t, "latest-run.json", v)
	s.note("run-snapshot", v)
	return v
}
func (s *stabilityRun) wait(label string, timeout time.Duration, predicate func(contracts.RunView) bool) contracts.RunView {
	s.stage(label)
	until := time.Now().Add(timeout)
	if deadline, ok := s.t.Deadline(); ok && until.After(deadline.Add(-2*time.Minute)) {
		until = deadline.Add(-2 * time.Minute)
	}
	for time.Now().Before(until) {
		v := s.view()
		if predicate(v) {
			return v
		}
		if v.Status == "waiting_input" || v.Status == "failed" || v.Status == "stopped" || v.Status == "blocked" && !v.ResumeRequired {
			s.t.Fatalf("unexpected execution failure: %s %s", v.Status, v.Diagnostic)
		}
		time.Sleep(2 * time.Second)
	}
	s.t.Fatalf("timed out at %s; original fixture retained", label)
	return contracts.RunView{}
}
func (s *stabilityRun) cli(args ...string) json.RawMessage {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := s.diagnostic(ctx, args...)
	v, parseErr := orca.ParseEnvelope(out)
	if err != nil || parseErr != nil || v.Meta.RuntimeID != s.m.RemoteRuntimeID {
		s.t.Fatalf("original diagnostic %v: %v: %s", args, err, out)
	}
	s.note("diagnostic", map[string]any{"args": args, "response": v.Result})
	return v.Result
}
func (s *stabilityRun) diagnostic(ctx context.Context, args ...string) ([]byte, error) {
	return livetest.DiagnosticRead(ctx, func(ctx context.Context, args ...string) ([]byte, error) {
		argv := append([]string(nil), args...)
		hasJSON := false
		for _, arg := range argv {
			if arg == "--json" {
				hasJSON = true
			}
		}
		argv = append(argv, "--environment", s.m.Execution.Target.Environment)
		if !hasJSON {
			argv = append(argv, "--json")
		}
		out, err := orca.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
			return exec.CommandContext(ctx, s.m.Stability.DiagnosticOrca, argv...).Output()
		})
		s.note("diagnostic-attempt", map[string]any{"args": args, "output": string(out), "error": fmt.Sprint(err)})
		v, parseErr := orca.ParseEnvelope(out)
		if v.OK && v.Meta.RuntimeID != s.m.RemoteRuntimeID {
			return out, errors.New("original diagnostic runtime changed")
		}
		return out, errors.Join(err, parseErr)
	}, args...)
}
func (s *stabilityRun) closeAuxiliary(original b.ProcessIdentity) error {
	s.note("auxiliary-close-intent", map[string]any{"intent_id": "close-" + original.Terminal, "original": original})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := orca.New(orca.Config{CommandTimeout: "30s", Output: func(ctx context.Context, _ time.Duration, args ...string) ([]byte, error) {
		return s.diagnostic(ctx, args...)
	}})
	err := livetest.CloseAuxiliaryTerminal(ctx, client, original)
	if err == nil {
		s.note("auxiliary-close-confirmed", original)
		delete(s.auxiliary, original.Terminal)
	}
	return err
}
func (s *stabilityRun) bind(v contracts.RunView) {
	parts := strings.Split(strings.TrimPrefix(v.Execution.SessionID, "orca-terminal:"+s.m.RemoteRuntimeID+":"), ":")
	if len(parts) != 2 {
		s.t.Fatal("original terminal identity unavailable")
	}
	var raw struct {
		Terminal orca.Terminal `json:"terminal"`
	}
	_ = json.Unmarshal(s.cli("terminal", "show", "--terminal", parts[0]), &raw)
	q := raw.Terminal
	s.check("terminal-binding", q.Handle == parts[0] && q.IncarnationID == parts[1] && q.WorktreePath == v.Execution.Workspace && (strings.TrimPrefix(q.Branch, "refs/heads/") == v.Execution.Branch || s.m.Scenario == "orca-remote.initialization-stop" && q.Branch == ""), "faults.jsonl: diagnostic terminal identity")
	s.process = b.ProcessIdentity{RuntimeID: s.m.RemoteRuntimeID, Terminal: q.Handle, Incarnation: q.IncarnationID, WorktreeID: q.WorktreeID}
	s.original = v.Execution
}
func (s *stabilityRun) native() (orca.AgentState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := orca.QueryState(ctx, s.m.Execution.Target.Environment, s.m.RemoteRuntimeID, "id:"+s.process.WorktreeID)
	if err != nil {
		return orca.AgentState{}, err
	}
	initial, err := orca.DecodeOriginalState(raw, s.process)
	if err != nil {
		return initial, err
	}
	state, err := orca.QueryOriginalAgent(ctx, s.m.Execution.Target.Environment, s.process, initial.PaneKey)
	if err == nil {
		s.note("original-native-state", state)
		if s.provider == "" {
			s.provider = state.ProviderSession.ID
			s.pane = state.PaneKey
		}
		if state.ProviderSession.ID != s.provider || state.PaneKey != s.pane {
			return state, fmt.Errorf("original provider/pane changed")
		}
	}
	return state, err
}
func (s *stabilityRun) nativeDone() {
	until := time.Now().Add(5 * time.Minute)
	for time.Now().Before(until) {
		v, err := s.native()
		if err == nil && livetest.MainTurnCompleted(v, s.workingAt, s.observedPrompt) {
			save(s.t, "native-completion-proof.json", v)
			return
		}
		time.Sleep(3 * time.Second)
	}
	s.t.Fatal("original native completion not verified")
}
func psquote(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }

type controlState struct {
	Entered   bool            `json:"entered"`
	Released  bool            `json:"released"`
	Published json.RawMessage `json:"published"`
	Audit     []struct {
		Kind string          `json:"kind"`
		Data json.RawMessage `json:"data"`
	} `json:"audit"`
}

func (s *stabilityRun) control(action, gate string) controlState {
	marker := fmt.Sprintf("SRCTL_%s_%d", s.m.ID, s.seq+1)
	code := "$ErrorActionPreference='Stop';try{$gd=(& git -C " + psquote(s.original.Workspace) + " rev-parse --absolute-git-dir|Out-String).Trim();if($LASTEXITCODE -ne 0){throw 'git admin unavailable'};$ctl=Join-Path " + psquote(s.original.Workspace) + " " + psquote(".sr-stability-"+s.m.ID) + ";New-Item -ItemType Directory -Force -Path $ctl|Out-Null;"
	if action == "release" {
		code += "[IO.File]::WriteAllText((Join-Path $ctl " + psquote(gate+".released") + "),'released');"
	}
	code += "$pub=$null;$pp=Join-Path $ctl 'published.json';if(Test-Path -LiteralPath $pp){$pub=Get-Content -LiteralPath $pp -Raw|ConvertFrom-Json};$audit=@();$ap=Join-Path $ctl 'audit.jsonl';if(Test-Path -LiteralPath $ap){$audit=@(Get-Content -LiteralPath $ap|ForEach-Object{$_|ConvertFrom-Json})};$v=@{entered=(Test-Path -LiteralPath (Join-Path $ctl " + psquote(gate+".entered") + "));released=(Test-Path -LiteralPath (Join-Path $ctl " + psquote(gate+".released") + "));published=$pub;audit=$audit};$json=@{ok=$true;result=$v}|ConvertTo-Json -Depth 12 -Compress}catch{$json=@{ok=$false;error=$_.Exception.Message}|ConvertTo-Json -Compress};Write-Output (('SR'+'CTL_'+" + psquote(s.m.ID+fmt.Sprintf("_%d", s.seq+1)) + ")+':'+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($json))+':ENDCTL')"
	units := utf16.Encode([]rune(code))
	encoded := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(encoded[i*2:], u)
	}
	var created struct {
		Terminal struct {
			Handle      string `json:"handle"`
			Incarnation string `json:"incarnationId"`
		} `json:"terminal"`
	}
	s.auxiliaryCreatePending = true
	s.note("auxiliary-create-intent", map[string]any{"worktree": s.process.WorktreeID, "marker": marker})
	_ = json.Unmarshal(s.cli("terminal", "create", "--worktree", "id:"+s.process.WorktreeID, "--title", "SteerRunner stability control", "--command", "powershell.exe -NoProfile -EncodedCommand "+base64.StdEncoding.EncodeToString(encoded)), &created)
	if created.Terminal.Handle == "" || created.Terminal.Incarnation == "" {
		s.t.Fatal("control terminal creation unknown; not recreated")
	}
	originalAux := b.ProcessIdentity{RuntimeID: s.m.RemoteRuntimeID, Terminal: created.Terminal.Handle, Incarnation: created.Terminal.Incarnation, WorktreeID: s.process.WorktreeID}
	if s.auxiliary == nil {
		s.auxiliary = map[string]b.ProcessIdentity{}
	}
	s.auxiliary[originalAux.Terminal] = originalAux
	s.auxiliaryCreatePending = false
	s.note("auxiliary-created-identity", originalAux)
	defer func() {
		if err := s.closeAuxiliary(originalAux); err != nil {
			s.t.Error("control terminal close unconfirmed", err)
		}
	}()
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		var shown struct {
			Terminal struct {
				Preview     string `json:"preview"`
				Incarnation string `json:"incarnationId"`
			} `json:"terminal"`
		}
		_ = json.Unmarshal(s.cli("terminal", "show", "--terminal", created.Terminal.Handle), &shown)
		if shown.Terminal.Incarnation != created.Terminal.Incarnation {
			s.t.Fatal("control terminal changed incarnation")
		}
		text := shown.Terminal.Preview
		if !strings.Contains(text, marker+":") || !strings.Contains(text, ":ENDCTL") {
			var read struct {
				Terminal struct {
					Handle string   `json:"handle"`
					Tail   []string `json:"tail"`
				} `json:"terminal"`
			}
			_ = json.Unmarshal(s.cli("terminal", "read", "--terminal", created.Terminal.Handle, "--limit", "400"), &read)
			if read.Terminal.Handle != created.Terminal.Handle {
				s.t.Fatal("control read handle changed")
			}
			text += "\n" + strings.Join(read.Terminal.Tail, "\n")
		}
		if i := strings.Index(text, marker+":"); i >= 0 {
			tail := text[i+len(marker)+1:]
			if j := strings.Index(tail, ":ENDCTL"); j >= 0 {
				payload, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(tail[:j]), ""))
				if err != nil {
					s.t.Fatal(err)
				}
				var result struct {
					OK     bool         `json:"ok"`
					Result controlState `json:"result"`
					Error  string       `json:"error"`
				}
				if json.Unmarshal(payload, &result) != nil || !result.OK {
					s.t.Fatalf("control failed: %s", payload)
				}
				return result.Result
			}
		}
		time.Sleep(time.Second)
	}
	s.t.Fatal("control action completion unknown")
	return controlState{}
}
func (s *stabilityRun) gate(stage string) {
	s.stage("await-" + stage + "-gate")
	until := time.Now().Add(12 * time.Minute)
	for time.Now().Before(until) {
		v := s.view()
		if v.Status == "blocked" || v.Status == "waiting_input" {
			s.t.Fatalf("native gate blocked: %s", v.Diagnostic)
		}
		if v.Execution.SessionID != "" && v.Execution.Workspace != "" && v.Execution.TurnStarted {
			if s.process.Terminal == "" {
				s.bind(v)
			}
			c := s.control("read", stage)
			if c.Entered {
				state, err := s.native()
				s.check("gate-native-working", err == nil && state.MainAgent != nil && state.MainAgent.State == "working", "faults.jsonl: checker gate and original main agent working")
				s.workingAt = state.MainAgent.StateStartedAt
				s.observedPrompt = state.Prompt
				return
			}
		}
		time.Sleep(5 * time.Second)
	}
	s.t.Fatal("gate never entered")
}
func (s *stabilityRun) network(action string) {
	s.note("network-intent", action)
	if action == "block" || action == "block-state" {
		s.blocked = true
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", s.m.Stability.Firewall.Path, "-Manifest", *manifestFlag, "-Action", action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("network %s: %v: %s", action, err, out)
	}
	if action == "restore" {
		s.blocked = false
	}
	s.note("network-confirmed", action)
}
func (s *stabilityRun) disconnect() {
	s.network("block")
	s.disconnectedAt = time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, s.m.Stability.Helper.Path, "probe-state", "-manifest", *manifestFlag, "-worktree", "id:"+s.process.WorktreeID).CombinedOutput()
	s.note("blocked-state-probe", map[string]any{"error": fmt.Sprint(err), "output": string(out)})
	s.check("direct-state-blocked", err != nil, "faults.jsonl: fixture helper direct native state request failed")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel2()
	out, err = orca.SerializeCLI(ctx2, func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, s.m.Stability.NativeOrca.Path, s.m.Stability.CLIEntry.Path, "terminal", "show", "--terminal", s.process.Terminal, "--environment", s.m.Execution.Target.Environment, "--json").CombinedOutput()
	})
	s.note("blocked-cli-probe", map[string]any{"error": fmt.Sprint(err), "output": string(out)})
	s.check("native-cli-blocked", err != nil, "faults.jsonl: real private CLI request failed")
	state, e := s.native()
	s.check("remote-still-live", e == nil && state.ProviderSession.ID == s.provider, "faults.jsonl: independent original runtime still queryable")
	// Keep the frozen fault window and prove the service itself completed a
	// failed observation and tried the original terminal again while offline.
	until := s.disconnectedAt.Add(time.Minute)
	for time.Now().Before(until) {
		s.view()
		time.Sleep(2 * time.Second)
	}
	raw, readErr := os.ReadFile(filepath.Join(*evidenceFlag, "native-calls.jsonl"))
	s.check("failed-observation-retried", readErr == nil && livetest.FailedObservationRetried(raw, s.process.Terminal, s.disconnectedAt), "native-calls.jsonl: failed original terminal read followed by a later original read during the 60-second network window")
	save(s.t, "network-observation-proof.json", livetest.ConnectionProof{StartedAt: s.disconnectedAt, ObservedAt: time.Now().UTC(), Terminal: s.process.Terminal})
	s.r.FaultsHit++
	s.check("fault-hit", true, "network-blocked.json and dual-path failure probes")
}
func (s *stabilityRun) startService() {
	s.serviceStartAttempted = true
	r := invoke(s.t, s.m, s.dir, []string{"service", "start"}, nil)
	raw, _ := json.Marshal(r.Data)
	if json.Unmarshal(raw, &s.instance) != nil || s.instance.InstanceID == "" {
		s.t.Fatal("service identity unavailable")
	}
	s.note("service-instance", s.instance)
}
func (s *stabilityRun) sameService() {
	response := invoke(s.t, s.m, s.dir, []string{"service", "status"}, nil)
	raw, _ := json.Marshal(response.Data)
	var actual contracts.ServiceInfo
	err := json.Unmarshal(raw, &actual)
	s.note("observed-original-service", actual)
	s.check("original-service-unchanged", err == nil && actual.InstanceID == s.instance.InstanceID && actual.PID == s.instance.PID && actual.ProcessCreated == s.instance.ProcessCreated, "caller-commands.jsonl: public service status retains original instance, PID and creation time")
}
func (s *stabilityRun) stopService(kill bool) {
	old := s.instance
	s.note("service-stop-intent", old)
	if kill {
		code := "$p=Get-Process -Id " + fmt.Sprint(old.PID) + " -ErrorAction Stop;if($p.Path -ine " + psquote(s.m.RunnerExe) + " -or $p.StartTime.ToUniversalTime().ToFileTimeUtc().ToString() -ne " + psquote(old.ProcessCreated) + "){throw 'service process identity mismatch'};Stop-Process -Id " + fmt.Sprint(old.PID) + " -ErrorAction Stop"
		out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", code).CombinedOutput()
		if err != nil {
			s.t.Fatalf("exact service termination failed: %v %s", err, out)
		}
	} else {
		req := request("service-stop-"+old.InstanceID, contracts.ServiceStopInput{ExpectedInstanceID: old.InstanceID})
		invoke(s.t, s.m, s.dir, []string{"service", "stop", "--input", "-"}, &req)
	}
	until := time.Now().Add(120 * time.Second)
	for time.Now().Before(until) {
		r, err := command(s.m, s.dir, []string{"service", "status"}, nil)
		if err == nil {
			raw, _ := json.Marshal(r.Data)
			var v contracts.ServiceInfo
			_ = json.Unmarshal(raw, &v)
			if v.Status == "not_running" {
				s.note("service-stopped", old)
				return
			}
		}
		time.Sleep(time.Second)
	}
	s.t.Fatal("old service exit not confirmed")
}
func (s *stabilityRun) restart(kill bool) {
	authority := s.view()
	old := s.instance.InstanceID
	s.stopService(kill)
	s.startService()
	s.check("service-replaced", old != s.instance.InstanceID, "faults.jsonl: old service ended and new instance started")
	s.r.FaultsHit++
	s.check("fault-hit", true, "faults.jsonl: original service ended and replacement instance confirmed")
	v := s.wait("restart-original-authority", 2*time.Minute, func(v contracts.RunView) bool {
		return v.ResumeRequired == authority.ResumeRequired && v.NewInputsAllowed == authority.NewInputsAllowed && v.Execution.AttemptID == authority.Execution.AttemptID && v.Execution.Generation == authority.Execution.Generation
	})
	s.originalIdentity(v)
	s.check("restart-preserves-authorization", v.ResumeRequired == authority.ResumeRequired && v.NewInputsAllowed == authority.NewInputsAllowed, "faults.jsonl: service generation replacement preserves the original authorization, including explicit stop revocation")
}
func (s *stabilityRun) write(commandName string, v contracts.RunView) {
	for attempt := 0; attempt < 5; attempt++ {
		req := request(fmt.Sprintf("%s-%d-revision-%d", commandName, s.seq, v.Revision), contracts.RevisionInput{ExpectedRevision: v.Revision})
		response, err := command(s.m, s.dir, []string{commandName, "--run-id", s.r.RunID, "--input", "-"}, &req)
		if err == nil {
			s.note("explicit-driver-authorization", map[string]any{"command": commandName, "request": req})
			return
		}
		// Only an explicit stale-revision rejection proves no control was
		// accepted. Unknown/lost replies retain the original request identity.
		if response.OK || response.Error == nil || response.Error.Code != "stale_revision" {
			s.t.Fatal("original control outcome unverified; no retry", err)
		}
		s.note("explicit-stale-control-rejection", map[string]any{"request": req, "revision": v.Revision})
		v = s.view()
		s.originalIdentity(v)
	}
	s.t.Fatal("original control revision did not stabilize within five explicit rejections")
}
func (s *stabilityRun) calls() ([]string, int) {
	raw, err := os.ReadFile(filepath.Join(*evidenceFlag, "native-calls.jsonl"))
	if err != nil {
		s.t.Fatal("missing native call audit", err)
	}
	facts, err := livetest.AnalyzeNativeAudit(raw, s.m.RemoteRuntimeID, livetest.ReceiptAuditPolicy(s.m))
	if err != nil {
		s.t.Fatal(err)
	}
	return facts.Prompts, facts.Creations
}
func (s *stabilityRun) noMoreInputs(count int) {
	keys, _ := s.calls()
	s.check("no-input-after-revocation-or-before-resume", len(keys) == count, "native-calls.jsonl: native send count unchanged during fenced window")
}
func (s *stabilityRun) originalIdentity(v contracts.RunView) {
	s.check("original-identity", v.Execution.SessionID == s.original.SessionID && v.Execution.Workspace == s.original.Workspace && v.Execution.Branch == s.original.Branch, "latest-run.json and faults.jsonl: original session/workspace/branch")
}
func (s *stabilityRun) merge(v contracts.RunView) {
	s.stage("verify-original-pr")
	verifyHead(s.t, s.m, v, s.started.Add(-time.Minute))
	s.r.PRURL = v.PR.URL
	rawList := shell(s.t, s.m.Root, "gh", "pr", "list", "--repo", livetest.Repository, "--base", s.m.BaseBranch, "--state", "all", "--json", "url")
	var prList []struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(rawList, &prList) != nil {
		s.t.Fatal("actual PR list invalid")
	}
	save(s.t, "pr-list.json", prList)
	s.check("single-actual-pr", len(prList) == 1 && prList[0].URL == v.PR.URL, "pr-list.json: exactly one PR on this fixture base")
	fresh, err := inspect(s.m, s.dir, s.r.RunID, true)
	if err != nil {
		s.t.Fatal(err)
	}
	if err = livetest.CheckPR(s.m, fresh); err != nil {
		s.t.Fatal(err)
	}
	s.check("merge-head-stable", fresh.PR.HeadRevision == v.PR.HeadRevision && fresh.PR.URL == v.PR.URL, "original PR verified before exact-head merge")
	s.auditPermissions()
	shell(s.t, s.m.Root, "gh", "pr", "merge", v.PR.URL, "--merge", "--match-head-commit", v.PR.HeadRevision)
	merged, err := inspect(s.m, s.dir, s.r.RunID, true)
	if err != nil {
		s.t.Fatal(err)
	}
	save(s.t, "merged-run.json", merged)
	if merged.PR == nil || !merged.PR.Merged || merged.PR.QueryError != "" || merged.PR.MergeCommit == nil || merged.PR.MergedAt == nil || merged.PR.URL != v.PR.URL || merged.PR.HeadRevision != v.PR.HeadRevision {
		s.t.Fatal("actual merged PR not verified")
	}
	s.r.MergeCommit = *merged.PR.MergeCommit
	git(s.t, s.m, "fetch", "--no-tags", "origin", "+refs/heads/"+s.m.BaseBranch+":refs/remotes/origin/"+s.m.BaseBranch)
	s.check("merged-base", strings.TrimSpace(string(git(s.t, s.m, "rev-parse", "refs/remotes/origin/"+s.m.BaseBranch))) == s.r.MergeCommit, "actual base fetch matches merge commit")
	git(s.t, s.m, "merge-base", "--is-ancestor", v.PR.HeadRevision, s.r.MergeCommit)
	s.check("merged-content", bytes.Equal(git(s.t, s.m, "show", s.r.MergeCommit+":"+livetest.Artifact), git(s.t, s.m, "show", v.PR.HeadRevision+":"+livetest.Artifact)), "merged Git object equals verified head")
	checkout := filepath.Join(s.m.Root, "merged")
	git(s.t, s.m, "worktree", "add", "--detach", checkout, s.r.MergeCommit)
	shell(s.t, checkout, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(checkout, "sr-smoke", "check.ps1"))
}

func executionStability(t *testing.T, m livetest.Manifest) {
	s := &stabilityRun{t: t, m: m, dir: filepath.Join(m.Root, "s"), started: time.Now().UTC(), r: livetest.StabilityResult{Scenario: m.Scenario, CaseID: m.CaseID, RepeatIndex: m.RepeatIndex, Stage: "preflight", TestOutcome: "failed"}}
	var inputsBeforeCleanup []string
	t.Cleanup(func() {
		s.cleanup()
		// The fixed control actor deliberately remains live through business
		// completion. Require its actual closure only after Run stop/cleanup.
		keys, _ := s.calls()
		if inputsBeforeCleanup != nil {
			s.check("no-input-during-cleanup", reflect.DeepEqual(keys, inputsBeforeCleanup), "original business forwarding intents unchanged by stop/authentication/control cleanup")
		}
		s.r.AuditComplete = s.completeNativeReceipts(keys)
		s.check("native-receipts-complete", s.r.AuditComplete, "native-calls.jsonl and native-resource-audit.json after cleanup: original business receipts and original Orca terminal closure; no helper authentication proof")
		if err := livetest.ValidateRuntime(m); err != nil {
			t.Error(err)
		}
		if !t.Failed() {
			s.r.TestOutcome = "passed"
			if err := s.r.Validate(m); err != nil {
				s.r.TestOutcome = "failed"
				t.Error(err)
			} else {
				s.r.Stage = "completed"
			}
		}
		save(t, "scenario.json", s.r)
	})
	if err := livetest.ValidateRuntime(m); err != nil {
		t.Fatal(err)
	}
	if m.Scenario == "orca-remote.connection-reconnect" || m.Scenario == "orca-remote.publication-unknown" || strings.HasSuffix(m.CaseID, "boundary") || m.CaseID == "unreachable" {
		s.networkPreflight()
	}
	s.startService()
	spec, _ := os.ReadFile(m.SpecPath)
	plan, _ := os.ReadFile(m.PlanPath)
	a, z := string(spec), string(plan)
	input := contracts.RunRequest{Title: "SteerRunner L3 " + m.Scenario + "/" + m.CaseID, Correlation: map[string]string{"fixture_id": m.ID, "scenario": m.Scenario, "case": m.CaseID}, Repository: contracts.Repository{URL: livetest.RemoteURL, LocalPath: m.RepositoryPath, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Kind: "inline", Path: "smoke.spec.md", Content: &a}, Plan: contracts.Document{Kind: "inline", Path: "smoke.plan.md", Content: &z}}, Workflow: contracts.Workflow{ID: "development", Version: 2}, Execution: m.Execution, Publication: m.Publication}
	selectedWorkflow, workflowErr := m.RunWorkflow(nil)
	if workflowErr != nil {
		t.Fatal(workflowErr)
	}
	input.Workflow = selectedWorkflow
	input.Workflow.PromptOverrides = smokePromptOverrides()
	req := request(m.ID+"-submit", input)
	s.stage("submit")
	if m.Scenario == "orca-remote.caller-exit" {
		s.caller(req)
	} else {
		r := invoke(t, m, s.dir, []string{"submit", "--input", "-"}, &req)
		s.r.RunID = r.Data.(map[string]any)["run_id"].(string)
	}
	if m.Scenario == "orca-remote.initialization-stop" {
		s.initializationStop()
		return
	}
	if m.Scenario != "orca-remote.caller-exit" {
		stage := "implementation"
		if m.Scenario == "orca-remote.publication-unknown" {
			stage = "publication"
		}
		s.gate(stage)
	}
	var before []string
	if m.Scenario != "orca-remote.caller-exit" {
		before, _ = s.calls()
	}
	switch m.Scenario {
	case "orca-remote.caller-exit":
		v := s.wait("caller-original-execution", 14*time.Minute, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
		s.bind(v)
		if _, err := s.native(); err != nil {
			t.Fatal(err)
		}
		s.merge(v)
	case "orca-remote.connection-reconnect":
		if m.CaseID == "pre-send-disconnect" {
			s.control("release", "implementation")
			s.preSendDisconnect()
		} else {
			s.disconnect()
			s.sameService()
			s.noMoreInputs(len(before))
			s.network("restore")
			s.control("release", "implementation")
		}
		v := s.wait("reconnected-original-workflow", 14*time.Minute, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
		s.sameService()
		s.check("normal-reconnect-no-resume", !v.ResumeRequired && v.Execution.AttemptID == s.original.AttemptID && v.Execution.Generation == s.original.Generation, "latest-run.json: original Attempt/generation retained without driver resume")
		s.originalIdentity(v)
		s.merge(v)
	case "orca-remote.service-restart":
		boundary := strings.HasSuffix(m.CaseID, "boundary")
		if boundary {
			s.disconnect()
			s.control("release", "implementation")
			s.nativeDone()
		}
		s.restart(strings.HasPrefix(m.CaseID, "kill"))
		s.noMoreInputs(len(before))
		if boundary {
			s.network("restore")
		} else {
			s.control("release", "implementation")
		}
		v := s.wait("continued-original-workflow", 14*time.Minute, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
		s.check("restart-continues-without-resume", !v.ResumeRequired && v.NewInputsAllowed && v.Execution.AttemptID == s.original.AttemptID && v.Execution.Generation == s.original.Generation, "latest-run.json: original Attempt/generation continues without a resume command")
		after, _ := s.calls()
		once := true
		for _, prompt := range before {
			count := 0
			for _, observed := range after {
				if observed == prompt {
					count++
				}
			}
			once = once && count == 1
		}
		s.check("original-input-once", once, "native-calls.jsonl: each original pre-restart business input occurred exactly once")
		s.originalIdentity(v)
		s.merge(v)
	case "orca-remote.stop-revocation":
		if m.CaseID == "active" {
			response, err := command(m, s.dir, []string{"service", "verify-stopped"}, nil)
			save(t, "active-verification-refused.json", response)
			s.check("active-verifier-refuses-writer-lease", err != nil && response.Error != nil && response.Error.Code == "directory_conflict", "active-verification-refused.json")
		}
		boundary := m.CaseID == "boundary"
		unknown := m.CaseID == "unreachable"
		if boundary || unknown {
			s.disconnect()
		}
		if boundary {
			s.control("release", "implementation")
			s.nativeDone()
		}
		v := s.view()
		s.write("stop", v)
		if unknown {
			v = s.wait("unknown-stop-revocation", 2*time.Minute, func(v contracts.RunView) bool {
				return !v.NewInputsAllowed && v.StopStatus != "stopped" && v.Status == "reconciling"
			})
			save(t, "unknown-stop.json", v)
			s.restart(false)
			v = s.view()
			s.check("revocation-survives-restart", !v.NewInputsAllowed && v.ResumeRequired, "latest-run.json after unreachable stop and service replacement")
			s.noMoreInputs(len(before))
			s.originalIdentity(v)
			refused := false
			for attempt := 0; attempt < 5; attempt++ {
				v = s.view()
				reply := request(fmt.Sprintf("%s-revoked-feedback-%d", m.ID, attempt), contracts.ReplyInput{ExpectedRevision: v.Revision, Kind: "feedback", Text: "This revoked Run must not receive input."})
				response, err := command(m, s.dir, []string{"reply", "--run-id", s.r.RunID, "--input", "-"}, &reply)
				if err != nil && !response.OK && response.Error != nil && response.Error.Code == "stale_revision" {
					continue
				}
				refused = err != nil && !response.OK && response.Error != nil && response.Error.Code == "resume_required"
				break
			}
			s.check("revoked-feedback-rejected", refused && !s.view().NewInputsAllowed, "caller-commands.jsonl: current-version feedback explicitly refused with resume_required")
			stale := request(m.ID+"-stale-revoked-feedback", contracts.ReplyInput{ExpectedRevision: v.Revision - 1, Kind: "feedback", Text: "An old revision cannot restore input authority."})
			staleResponse, staleErr := command(m, s.dir, []string{"reply", "--run-id", s.r.RunID, "--input", "-"}, &stale)
			s.check("stale-revoked-write-rejected", staleErr != nil && !staleResponse.OK && staleResponse.Error != nil && staleResponse.Error.Code == "stale_revision" && !s.view().NewInputsAllowed, "caller-commands.jsonl: old revision explicitly refused without restoring authority")
			for time.Now().Before(s.disconnectedAt.Add(4 * time.Minute)) {
				v = s.view()
				s.check("unreachable-revocation-window", !v.NewInputsAllowed && v.ResumeRequired && v.StopStatus != "stopped", "faults.jsonl: revocation and unknown stop retained throughout the frozen 240-second outage")
				s.noMoreInputs(len(before))
				time.Sleep(2 * time.Second)
			}
			s.network("restore")
			s.r.BusinessOutcome = "revoked-unverified"
		} else {
			if boundary {
				s.network("restore")
			}
			v = s.wait("original-stopped", 2*time.Minute, func(v contracts.RunView) bool { return !v.NewInputsAllowed && v.StopStatus == "stopped" })
			s.originalIdentity(v)
			s.noMoreInputs(len(before))
			s.r.BusinessOutcome = "stopped"
			if !boundary {
				s.r.FaultsHit++
				s.check("fault-hit", true, "faults.jsonl: active checker stopped by original resource close")
			}
		}
		raw := shell(t, m.Root, "gh", "pr", "list", "--repo", livetest.Repository, "--base", m.BaseBranch, "--state", "all", "--json", "url")
		var prs []any
		if json.Unmarshal(raw, &prs) != nil {
			t.Fatal("invalid actual PR list for stop case")
		}
		save(t, "pr-list.json", prs)
		s.check("no-unexpected-publication", len(prs) == 0, "driver-commands.jsonl: no PR for dedicated stop fixture")
	case "orca-remote.publication-unknown":
		published := s.control("read", "publication")
		save(t, "publication-before-gap.json", published)
		s.check("real-publication-before-gap", len(published.Published) > 4, "publication-before-gap.json: actual PR identity")
		var originalPR struct {
			URL  string `json:"url"`
			Head string `json:"headRefOid"`
		}
		if json.Unmarshal(published.Published, &originalPR) != nil || originalPR.URL == "" || originalPR.Head == "" {
			t.Fatal("original published identity missing")
		}
		s.disconnect()
		s.control("release", "publication")
		s.nativeDone()
		s.check("completion-gap-unverified", s.view().Status != "pr_ready", "latest-run.json: no verified completion while disconnected")
		sentinel := filepath.Join(m.Root, "bin", "query-unavailable")
		if m.CaseID == "query-unavailable" {
			if err := os.WriteFile(sentinel, []byte("injected"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		s.network("restore")
		if m.CaseID == "query-unavailable" {
			s.wait("provider-query-failure-observed", 2*time.Minute, func(v contracts.RunView) bool {
				raw, err := os.ReadFile(filepath.Join(*evidenceFlag, "provider-queries.jsonl"))
				if err != nil {
					return false
				}
				hit := false
				for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
					var entry struct {
						Injected bool   `json:"injected"`
						URL      string `json:"url"`
					}
					if json.Unmarshal(line, &entry) == nil && entry.Injected && entry.URL == originalPR.URL {
						hit = true
					}
				}
				return hit && v.Status == "reconciling" && v.Phase == "pr_unverified"
			})
			s.r.FaultsHit++
			s.check("query-failure-no-false-ready", s.view().Status != "pr_ready", "provider-queries.jsonl and latest-run.json: actual injected query failed and original binding remains unverified")
			if err := os.Remove(sentinel); err != nil {
				t.Fatal(err)
			}
		}
		v := s.wait("reconciled-original-publication", 14*time.Minute, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
		s.originalIdentity(v)
		s.check("original-publication-binding", v.PR != nil && v.PR.URL == originalPR.URL && v.PR.HeadRevision == originalPR.Head, "publication-before-gap.json and latest-run.json: original PR and head preserved")
		s.merge(v)
	default:
		t.Fatal("unregistered stability scenario")
	}
	if m.Stability.ExpectedOutcome == "merged" {
		s.r.BusinessOutcome = "merged"
	}
	keys, launches := s.calls()
	inputsBeforeCleanup = append([]string{}, keys...)
	seen := map[string]bool{}
	unique := true
	for _, k := range keys {
		if seen[k] {
			unique = false
		}
		seen[k] = true
	}
	s.check("no-duplicate-inputs", unique && launches == 1, "native-calls.jsonl: unique business sends and one business worktree/terminal; exact original readonly inspectors counted separately")
	if m.Scenario == "orca-remote.caller-exit" {
		s.originalIdentity(s.view())
	}
	if m.Stability.ExpectedOutcome == "merged" {
		c := s.control("read", "publication")
		save(t, "native-publication-audit.json", c)
		pubs, creates := 0, 0
		for _, a := range c.Audit {
			if a.Kind == "publisher" {
				pubs++
			}
			if a.Kind == "create" {
				creates++
			}
		}
		s.check("single-native-publication", pubs == 1 && creates == 1, "native-publication-audit.json: publisher/create counts")
	}
	if m.Stability.ExpectedOutcome != "merged" {
		s.auditPermissions()
	}
	s.stage("cleanup")
}

func (s *stabilityRun) preSendDisconnect() {
	s.stage("await-publication-pre-send")
	gatePath := filepath.Join(s.m.Root, "evidence", "pre-send-gate.json")
	until := time.Now().Add(5 * time.Minute)
	var gate livetest.PreSendGate
	for time.Now().Before(until) {
		raw, err := os.ReadFile(gatePath)
		if err == nil && json.Unmarshal(raw, &gate) == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.check("publication-pre-send-gate", gate.View.RunID == s.r.RunID && gate.Terminal == s.process.Terminal && livetest.PublicationPreSend(gate.View, gate.Terminal), "pre-send-gate.json: committed publish input before direct state connect")
	s.originalIdentity(gate.View)
	s.network("block-state")
	if err := os.WriteFile(filepath.Join(s.m.Root, "evidence", "pre-send-released"), []byte("released"), 0600); err != nil {
		s.t.Fatal(err)
	}
	until = time.Now().Add(20 * time.Second)
	var raw []byte
	var retried bool
	for time.Now().Before(until) {
		returned, err := os.ReadFile(filepath.Join(s.m.Root, "evidence", "pre-send-returned.json"))
		if err == nil && json.Unmarshal(returned, &gate) == nil {
			raw, _ = os.ReadFile(filepath.Join(s.m.Root, "evidence", "native-calls.jsonl"))
			if livetest.PreSendReadRetried(raw, gate) {
				retried = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.check("pre-send-connection-retried", retried, "pre-send-returned.json and native-calls.jsonl: original readiness retried after a full failed direct dial; zero new sends")
	if err := os.WriteFile(filepath.Join(s.m.Root, "evidence", "pre-send-retry-audit.jsonl"), raw, 0600); err != nil {
		s.t.Fatal(err)
	}
	v := s.view()
	s.check("pre-send-authority-retained", v.Status == "running" && v.NewInputsAllowed && !v.ResumeRequired && len(v.Operations) > 0 && v.Operations[len(v.Operations)-1].ID == gate.View.Operations[len(gate.View.Operations)-1].ID, "pre-send-blocked-run.json: original operation/authority retained")
	save(s.t, "pre-send-blocked-run.json", v)
	s.r.FaultsHit++
	s.check("fault-hit", true, "Runner-only direct connection blocked after original publish identity read")
	s.network("restore")
}

func (s *stabilityRun) auditPermissions() {
	var settings contracts.OrcaSettings
	err := contracts.DecodeStrict(s.m.Execution.Settings, &settings)
	valid := err == nil && contracts.ValidateExecution(s.m.Execution) == nil && settings.AgentLaunch.RuntimeExecutable == "" && settings.AgentLaunch.RuntimeSHA256 == ""
	save(s.t, "native-launch-contract.json", map[string]any{"requested_model": settings.AgentLaunch.Model, "requested_reasoning_effort": settings.AgentLaunch.ReasoningEffort, "requested_access_mode": settings.AccessMode, "effective_parameters_verified": false})
	s.check("native-launch-parameters", valid, "native-launch-contract.json: frozen requested parameters; no effective-parameter or helper audit")
}

func (s *stabilityRun) completeNativeReceipts(expected []string) bool {
	// Run cleanup already stopped the original business/control resources.
	// This loop only awaits their existing bound receipts, with no new effects.
	until := time.Now().Add(30 * time.Second)
	for {
		raw, readErr := os.ReadFile(filepath.Join(*evidenceFlag, "native-calls.jsonl"))
		facts, err := livetest.AnalyzeNativeAudit(raw, s.m.RemoteRuntimeID, livetest.ReceiptAuditPolicy(s.m))
		verified := readErr == nil && err == nil && facts.Complete && facts.Creations == 1 && reflect.DeepEqual(facts.Prompts, expected)
		if verified || !time.Now().Before(until) {
			save(s.t, "native-resource-audit.json", map[string]any{"verified": verified, "business_creations": facts.Creations, "initial_terminal_creations": facts.InitialTerminals, "prompt_sends": len(facts.Prompts), "inspections": facts.Inspections, "closed_inspections": facts.ClosedInspections})
			return verified
		}
		time.Sleep(200 * time.Millisecond)
	}
}
func (s *stabilityRun) caller(req contracts.Request) {
	raw, _ := json.Marshal(req)
	path := filepath.Join(s.m.Root, "evidence", "caller-request.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		s.t.Fatal(err)
	}
	cmd := exec.Command(s.m.Stability.Helper.Path, "caller", "-manifest", *manifestFlag, "-request", path)
	if err := cmd.Start(); err != nil {
		s.t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	until := time.Now().Add(time.Minute)
	for time.Now().Before(until) {
		r, err := command(s.m, s.dir, []string{"inspect", "--request-id", req.RequestID}, nil)
		if err == nil {
			raw, _ := json.Marshal(r.Data)
			var data struct {
				Run *contracts.RunView `json:"run"`
			}
			_ = json.Unmarshal(raw, &data)
			if data.Run != nil {
				s.r.RunID = data.Run.RunID
				break
			}
		}
		time.Sleep(time.Second)
	}
	if s.r.RunID == "" {
		s.t.Fatal("accepted caller request not found")
	}
	if err := cmd.Process.Kill(); err != nil {
		s.t.Fatal("caller already exited; fault not hit", err)
	}
	s.note("caller-terminated", map[string]any{"pid": cmd.Process.Pid, "request_id": req.RequestID, "run_id": s.r.RunID})
	s.r.FaultsHit++
	s.check("fault-hit", true, "faults.jsonl: owned caller terminated after original request was accepted")
	status := invoke(s.t, s.m, s.dir, []string{"service", "status"}, nil)
	rawStatus, _ := json.Marshal(status.Data)
	var service contracts.ServiceInfo
	_ = json.Unmarshal(rawStatus, &service)
	s.check("caller-service-survives", service.InstanceID == s.instance.InstanceID && service.Status == "ready", "faults.jsonl and caller-commands.jsonl: same service survived caller termination")
	r := invoke(s.t, s.m, s.dir, []string{"submit", "--input", "-"}, &req)
	s.check("submit-replay-original", r.Data.(map[string]any)["run_id"] == s.r.RunID, "caller-commands.jsonl: original request replay returns original Run")
}

func (s *stabilityRun) networkPreflight() {
	s.stage("dual-path-network-preflight")
	var repos struct {
		Repos []struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Identity struct {
				URL string `json:"remoteUrl"`
			} `json:"gitRemoteIdentity"`
		} `json:"repos"`
	}
	_ = json.Unmarshal(s.cli("repo", "list"), &repos)
	repoID := ""
	for _, r := range repos.Repos {
		if strings.HasSuffix(strings.ReplaceAll(r.Path, "\\", "/"), "/SteerlineTest") && r.Identity.URL == livetest.RemoteURL {
			repoID = r.ID
		}
	}
	if repoID == "" {
		s.t.Fatal("network preflight repository identity unavailable")
	}
	var listing struct {
		Worktrees []struct {
			ID   string `json:"id"`
			Main bool   `json:"isMainWorktree"`
		} `json:"worktrees"`
	}
	_ = json.Unmarshal(s.cli("worktree", "list", "--repo", "id:"+repoID), &listing)
	mainID := ""
	for _, w := range listing.Worktrees {
		if w.Main {
			mainID = w.ID
		}
	}
	if mainID == "" {
		s.t.Fatal("network preflight main worktree unavailable")
	}
	cmd := exec.Command(s.m.Stability.Helper.Path, "probe-subscribe", "-manifest", *manifestFlag, "-worktree", "id:"+mainID)
	if err := cmd.Start(); err != nil {
		s.t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	opened := filepath.Join(s.m.Root, "evidence", "probe-subscription-open")
	disconnected := filepath.Join(s.m.Root, "evidence", "probe-subscription-disconnected")
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		if _, err := os.Stat(opened); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if _, err := os.Stat(opened); err != nil {
		s.t.Fatal("real preflight subscription did not open")
	}
	s.network("block")
	until = time.Now().Add(20 * time.Second)
	for time.Now().Before(until) {
		if _, err := os.Stat(disconnected); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if _, err := os.Stat(disconnected); err != nil {
		s.t.Fatal("existing native subscription did not disconnect")
	}
	for _, args := range [][]string{{"probe-state", "-manifest", *manifestFlag, "-worktree", "id:" + mainID}, {"audit", "-manifest", *manifestFlag, "--", "repo", "list", "--environment", s.m.Execution.Target.Environment, "--json"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		out, err := exec.CommandContext(ctx, s.m.Stability.Helper.Path, args...).CombinedOutput()
		cancel()
		s.note("preflight-blocked-request", map[string]any{"args": args, "error": fmt.Sprint(err), "output": string(out)})
		if err == nil {
			s.t.Fatal("network preflight path remained reachable")
		}
	}
	s.network("restore")
	for _, args := range [][]string{{"probe-state", "-manifest", *manifestFlag, "-worktree", "id:" + mainID}, {"audit", "-manifest", *manifestFlag, "--", "repo", "list", "--environment", s.m.Execution.Target.Environment, "--json"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, err := exec.CommandContext(ctx, s.m.Stability.Helper.Path, args...).CombinedOutput()
		cancel()
		s.note("preflight-restored-request", map[string]any{"args": args, "error": fmt.Sprint(err), "output": string(out)})
		if err != nil {
			s.t.Fatal("network preflight path did not recover", err, string(out))
		}
	}
	s.check("network-preflight", true, "probe-subscription-disconnected and faults.jsonl: existing stream cut, both paths blocked/restored before model startup")
}

func (s *stabilityRun) cleanup() {
	if !s.serviceStartAttempted {
		s.r.Cleanup.NativeStopped = true
		s.r.Cleanup.ServiceStopped = true
	}
	if s.blocked {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", s.m.Stability.Firewall.Path, "-Manifest", *manifestFlag, "-Action", "restore")
		out, err := cmd.CombinedOutput()
		if err != nil {
			s.t.Errorf("network cleanup: %v %s", err, out)
			return
		}
		s.blocked = false
	}
	s.r.Cleanup.FacilitiesRestored = true
	for _, auxiliary := range s.auxiliary {
		if err := s.closeAuxiliary(auxiliary); err != nil {
			s.r.Cleanup.FacilitiesRestored = false
			s.t.Error("auxiliary cleanup unresolved", err)
		}
	}
	if s.auxiliaryCreatePending {
		s.r.Cleanup.FacilitiesRestored = false
		s.t.Error("auxiliary creation outcome unknown; preserve original intent")
	}
	if _, err := os.Stat(filepath.Join(s.m.Root, "evidence", "network-watchdog-expired.json")); err == nil {
		s.r.Cleanup.FacilitiesRestored = false
		s.t.Error("network watchdog expired unexpectedly; original case failed")
	}
	_ = os.Remove(filepath.Join(s.m.Root, "bin", "query-unavailable"))
	if s.r.RunID == "" {
		r, err := command(s.m, s.dir, []string{"inspect", "--request-id", s.m.ID + "-submit"}, nil)
		if err == nil {
			raw, _ := json.Marshal(r.Data)
			var data struct {
				Run *contracts.RunView `json:"run"`
			}
			_ = json.Unmarshal(raw, &data)
			if data.Run != nil {
				s.r.RunID = data.Run.RunID
			}
		}
	}
	if s.r.RunID != "" {
		v, err := inspect(s.m, s.dir, s.r.RunID, false)
		if err == nil {
			if v.StopStatus != "stopped" {
				err = revokeOriginalRun(s.m, s.dir, s.r.RunID, "cleanup-run")
			}
			if err == nil {
				until := time.Now().Add(75 * time.Second)
				for time.Now().Before(until) {
					v, err = inspect(s.m, s.dir, s.r.RunID, false)
					if err == nil && v.StopStatus == "stopped" && !v.NewInputsAllowed {
						s.r.Cleanup.NativeStopped = true
						save(s.t, "cleanup-run.json", v)

						break
					}
					time.Sleep(time.Second)
				}
			}
		}
		if !s.r.Cleanup.NativeStopped && s.m.CaseID == "unreachable" && !v.NewInputsAllowed && s.process.Terminal != "" {
			var original struct {
				Terminal orca.Terminal `json:"terminal"`
			}
			_ = json.Unmarshal(s.cli("terminal", "show", "--terminal", s.process.Terminal), &original)
			if original.Terminal.IncarnationID == s.process.Incarnation && original.Terminal.WorktreeID == s.process.WorktreeID {
				s.note("driver-original-cleanup-intent", s.process)
				var closed struct {
					Close struct {
						Handle string `json:"handle"`
						Killed bool   `json:"ptyKilled"`
					} `json:"close"`
				}
				_ = json.Unmarshal(s.cli("terminal", "close", "--terminal", s.process.Terminal), &closed)
				if closed.Close.Handle == s.process.Terminal && closed.Close.Killed {
					s.r.Cleanup.NativeStopped = true
					s.r.Cleanup.DriverNativeCleanup = true
					save(s.t, "runner-stop-still-unverified.json", v)
				}
			}
		}
		if !s.r.Cleanup.NativeStopped {
			s.t.Errorf("original native stop not confirmed: %v", err)
		}
	}
	if s.instance.InstanceID != "" {
		req := request("cleanup-service", contracts.ServiceStopInput{ExpectedInstanceID: s.instance.InstanceID})
		_, err := command(s.m, s.dir, []string{"service", "stop", "--input", "-"}, &req)
		if err != nil {
			s.t.Error(err)
		} else {
			until := time.Now().Add(30 * time.Second)
			for time.Now().Before(until) {
				r, e := command(s.m, s.dir, []string{"service", "status"}, nil)
				if e == nil {
					raw, _ := json.Marshal(r.Data)
					var v contracts.ServiceInfo
					_ = json.Unmarshal(raw, &v)
					if v.Status == "not_running" {
						s.r.Cleanup.ServiceStopped = true
						break
					}
				}
				time.Sleep(time.Second)
			}
		}
	}
	if !s.r.Cleanup.ServiceStopped {
		s.t.Error("current service exit unconfirmed")
	}
	if s.r.Cleanup.ServiceStopped && s.r.Cleanup.NativeStopped && !s.r.Cleanup.DriverNativeCleanup && s.r.RunID != "" {
		proof, err := verifyNativeStoppedScope(s.m, s.dir, s.r.RunID)
		save(s.t, "offline-stopped.json", proof)
		if err != nil {
			s.t.Error(err)
		}
	}
	save(s.t, "cleanup.json", s.r.Cleanup)
}
