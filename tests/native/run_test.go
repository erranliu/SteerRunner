//go:build windows && live

package native

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

var manifestFlag = flag.String("native-manifest", "", "Runner-owned frozen live fixture")
var evidenceFlag = flag.String("evidence-dir", "", "Fresh evidence directory inside this fixture")
var scenarioCount int

// New real-environment workflows register a driver; catalog entries choose
// that driver and an execution target without changing the meaning of L3.
var drivers = map[string]func(*testing.T, livetest.Manifest){"environment-check": environmentCheck, "development-core": developmentSmoke, "development-smoke": developmentSmoke, "development-feedback": developmentSmoke, "development-audited": developmentSmoke, "development-multirun": developmentMultipleRuns, "execution-stability": executionStability}

func TestMain(m *testing.M) {
	flag.Parse()
	if *evidenceFlag == "" {
		fmt.Fprintln(os.Stderr, "explicit -evidence-dir is required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*evidenceFlag, 0700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	log, err := os.OpenFile(filepath.Join(*evidenceFlag, "native.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Stdout, os.Stderr = log, log
	code := m.Run()
	if scenarioCount == 0 {
		fmt.Fprintln(log, "No live scenario matched; zero execution cannot pass.")
		code = 1
	}
	body, _ := json.MarshalIndent(map[string]any{"exit_code": code, "scenario_count": scenarioCount, "finished_at": time.Now().UTC(), "pattern": flag.Lookup("test.run").Value.String()}, "", "  ")
	if err := process.New(0).WriteExecutionFile(filepath.Join(*evidenceFlag, "result.json"), body); err != nil {
		fmt.Fprintln(log, err)
		code = 1
	}
	_ = log.Close()
	os.Exit(code)
}

func TestSRLiveScenario(t *testing.T) {
	scenarioCount++
	body, err := os.ReadFile(*manifestFlag)
	if err != nil {
		t.Fatal(err)
	}
	var m livetest.Manifest
	if err := contracts.DecodeStrict(body, &m); err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(*evidenceFlag)
	if err != nil || !strings.EqualFold(root, filepath.Join(m.Root, "evidence")) {
		t.Fatal("evidence must be the generated fixture's own evidence directory")
	}
	driver, ok := drivers[m.Driver]
	if !ok {
		t.Fatalf("unregistered real-environment driver %q", m.Driver)
	}
	t.Run(m.Scenario, func(t *testing.T) { driver(t, m) })
}

type scenarioResult struct {
	Runs           []contracts.RunView `json:"runs,omitempty"`
	Scenario       string              `json:"scenario"`
	Level          string              `json:"level"`
	Stage          string              `json:"stage"`
	RunID          string              `json:"run_id,omitempty"`
	PRURL          string              `json:"pr_url,omitempty"`
	OriginalHead   string              `json:"original_head,omitempty"`
	FeedbackHead   string              `json:"feedback_head,omitempty"`
	MergeCommit    string              `json:"merge_commit,omitempty"`
	RunStopped     bool                `json:"run_stopped"`
	ServiceStopped bool                `json:"service_stopped"`
	Passed         bool                `json:"passed"`
}

func save(t *testing.T, name string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		err = process.New(0).WriteExecutionFile(filepath.Join(*evidenceFlag, name), body)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func command(m livetest.Manifest, dir string, args []string, input *contracts.Request) (contracts.Response, error) {
	argv := append(append([]string{}, args...), "--state-dir", dir, "--json")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.RunnerExe, argv...)
	var body []byte
	if input != nil {
		body, _ = json.Marshal(input)
		cmd.Stdin = bytes.NewReader(body)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	entry, _ := json.Marshal(map[string]any{"at": time.Now().UTC(), "argv": argv, "request": string(body), "response": string(out), "stderr": stderr.String()})
	f, logErr := os.OpenFile(filepath.Join(*evidenceFlag, "caller-commands.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if logErr != nil {
		return contracts.Response{}, logErr
	}
	_, logErr = f.Write(append(entry, '\n'))
	_ = f.Close()
	if logErr != nil {
		return contracts.Response{}, logErr
	}
	var response contracts.Response
	if json.Unmarshal(out, &response) != nil {
		return response, fmt.Errorf("CLI %v: %v: %s", args, err, stderr.String())
	}
	if err != nil || !response.OK {
		return response, fmt.Errorf("CLI %v: %v: %+v", args, err, response.Error)
	}
	return response, nil
}

func invoke(t *testing.T, m livetest.Manifest, dir string, args []string, input *contracts.Request) contracts.Response {
	t.Helper()
	response, err := command(m, dir, args, input)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func inspect(m livetest.Manifest, dir, id string, refresh bool) (contracts.RunView, error) {
	args := []string{"inspect", "--run-id", id}
	if refresh {
		args = append(args, "--refresh-pr")
	}
	r, err := command(m, dir, args, nil)
	if err != nil {
		return contracts.RunView{}, err
	}
	body, err := json.Marshal(r.Data)
	if err != nil {
		return contracts.RunView{}, err
	}
	var data struct {
		Run contracts.RunView `json:"run"`
	}
	err = json.Unmarshal(body, &data)
	return data.Run, err
}

// A stale revision proves rejection before acceptance. Only this explicit case
// permits a refreshed stop with a new request; unknown outcomes retain their ID.
func revokeOriginalRun(m livetest.Manifest, dir, id, prefix string) error {
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		view, err := inspect(m, dir, id, false)
		if err != nil {
			return err
		}
		if view.StopStatus == "stopped" && !view.NewInputsAllowed {
			return nil
		}
		req := request(fmt.Sprintf("%s-%d", prefix, attempt), contracts.RevisionInput{ExpectedRevision: view.Revision})
		response, err := command(m, dir, []string{"stop", "--run-id", id, "--input", "-"}, &req)
		if err == nil {
			return nil
		}
		last = err
		if response.Error == nil || response.Error.Code != "stale_revision" {
			return err
		}
	}
	return fmt.Errorf("stop revision remained unstable after explicit rejections: %w", last)
}

func shell(t *testing.T, dir, exe string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	entry, _ := json.Marshal(map[string]any{"at": time.Now().UTC(), "exe": exe, "args": args, "stdout": string(out), "stderr": stderr.String(), "failed": err != nil})
	f, e := os.OpenFile(filepath.Join(*evidenceFlag, "driver-commands.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.Write(append(entry, '\n'))
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if err != nil {
		t.Fatalf("%s %v: %v %s", exe, args, err, stderr.String())
	}
	return out
}

func git(t *testing.T, m livetest.Manifest, args ...string) []byte {
	t.Helper()
	return shell(t, m.RepositoryPath, "git", append([]string{"-c", "core.longpaths=true"}, args...)...)
}

func waitReady(t *testing.T, m livetest.Manifest, dir, id, oldHead string) contracts.RunView {
	t.Helper()
	until := time.Now().Add(12 * time.Minute)
	for time.Now().Before(until) {
		view, err := livetest.ObservePublicationReady(oldHead, func(refresh bool) (contracts.RunView, error) {
			return inspect(m, dir, id, refresh)
		})
		if err != nil {
			t.Fatal(err)
		}
		save(t, "latest-run.json", view)
		if view.Status == "pr_ready" && view.PR != nil && view.PR.HeadRevision != oldHead {
			return view
		}
		if view.Status == "blocked" || view.Status == "waiting_input" || view.Status == "stopped" || view.ResumeRequired {
			t.Fatalf("original execution needs intervention: status=%s phase=%s diagnostic=%s", view.Status, view.Phase, view.Diagnostic)
		}
		if view.Status == "reconciling" && strings.HasPrefix(view.Diagnostic, "unknown_external:") {
			t.Fatalf("original publication outcome needs read-only reconciliation: %s", view.Diagnostic)
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("original execution timed out; no replacement or resend was attempted")
	return contracts.RunView{}
}

func request(id string, value any) contracts.Request {
	body, _ := json.Marshal(value)
	return contracts.Request{Protocol: contracts.Protocol, RequestID: id, Input: body}
}

// Diagnostic reads are fenced to the original remote resource and never
// replace the Runner's native evidence or change business state.
func remoteDiagnostic(m livetest.Manifest, view contracts.RunView) ([]byte, error) {
	var settings contracts.OrcaSettings
	if err := contracts.DecodeStrict(m.Execution.Settings, &settings); err != nil {
		return nil, err
	}
	prefix := "orca-terminal:" + m.RemoteRuntimeID + ":"
	if !strings.HasPrefix(view.Execution.SessionID, prefix) {
		return nil, errors.New("original remote session identity is unavailable")
	}
	parts := strings.Split(strings.TrimPrefix(view.Execution.SessionID, prefix), ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errors.New("original terminal identity is incomplete")
	}
	args := append(append([]string{}, settings.Transport.Args...), "terminal", "show", "--terminal", parts[0], "--environment", m.Execution.Target.Environment, "--json")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, settings.Transport.Executable, args...).Output()
	if err != nil {
		return nil, err
	}
	var proof struct {
		OK   bool `json:"ok"`
		Meta struct {
			Runtime string `json:"runtimeId"`
		} `json:"_meta"`
		Result struct {
			Terminal struct {
				Handle      string `json:"handle"`
				Incarnation string `json:"incarnationId"`
			} `json:"terminal"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &proof) != nil || !proof.OK || proof.Meta.Runtime != m.RemoteRuntimeID || proof.Result.Terminal.Handle != parts[0] || proof.Result.Terminal.Incarnation != parts[1] {
		return nil, errors.New("remote diagnostic identity did not match the original terminal")
	}
	return out, nil
}

func verifyHead(t *testing.T, m livetest.Manifest, view contracts.RunView, earliest time.Time) time.Time {
	t.Helper()
	if err := livetest.CheckPR(m, view); err != nil {
		t.Fatal(err)
	}
	if len(view.Materials) != 2 {
		t.Fatal("Runner did not preserve both frozen documents")
	}
	wantMaterials := map[string]string{"smoke.spec.md": m.SpecSHA256, "smoke.plan.md": m.PlanSHA256}
	for _, material := range view.Materials {
		if wantMaterials[material.Path] != material.Digest {
			t.Fatal("Runner's frozen material differs from fixture", material.Path)
		}
		delete(wantMaterials, material.Path)
	}
	if len(wantMaterials) != 0 {
		t.Fatal("Runner's frozen material set is incomplete")
	}
	// Fetch server-side objects for both local and remote execution.
	git(t, m, "fetch", "--no-tags", "origin", fmt.Sprintf("+refs/pull/%d/head:refs/srtest/pr", view.PR.Number))
	head := strings.TrimSpace(string(git(t, m, "rev-parse", "refs/srtest/pr")))
	if head != view.PR.HeadRevision {
		t.Fatal("provider and fetched PR head differ")
	}
	changed := strings.TrimSpace(string(git(t, m, "diff", "--name-only", m.BaseRevision, head)))
	if changed != livetest.Artifact {
		t.Fatalf("implementation changed unexpected paths: %q", changed)
	}
	git(t, m, "diff", "--check", m.BaseRevision, head)
	body := git(t, m, "show", head+":"+livetest.Artifact)
	at, err := livetest.ValidateArtifact(body, earliest, time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(m.Root, "check-"+head[:12])
	if _, err := os.Stat(checkout); os.IsNotExist(err) {
		git(t, m, "worktree", "add", "--detach", checkout, head)
	} else if err != nil || string(bytes.TrimSpace(shell(t, checkout, "git", "rev-parse", "HEAD"))) != head {
		t.Fatal("verification checkout does not match the frozen PR head", err)
	}
	shell(t, checkout, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(checkout, "sr-smoke", "check.ps1"))
	return at
}

func developmentSmoke(t *testing.T, m livetest.Manifest) {
	audited := m.Driver == "development-audited"
	r := scenarioResult{Scenario: m.Scenario, Level: m.Level, Stage: "preflight"}
	checkpoint := func(stage string) { r.Stage = stage; save(t, "scenario.json", r) }
	// Registered first: final success is written after every cleanup ran.
	t.Cleanup(func() {
		r.Passed = !t.Failed() && r.RunStopped && r.ServiceStopped && (audited && r.OriginalHead != "" || !audited && r.MergeCommit != "")
		if r.Passed {
			r.Stage = "completed"
		}
		save(t, "scenario.json", r)
	})
	if got := strings.TrimSpace(string(git(t, m, "remote", "get-url", "origin"))); got != livetest.RemoteURL {
		t.Fatal("fixture repository remote changed")
	}
	if got := strings.TrimSpace(string(git(t, m, "rev-parse", "HEAD"))); got != m.BaseRevision {
		t.Fatal("fixture repository base changed")
	}
	if m.Level == "L2" {
		if _, err := os.Stat(filepath.Join(m.RepositoryPath, "sr-smoke", "publish.ps1")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("L2 must exercise production publication without the scripted publisher", err)
		}
	}
	base := strings.Fields(string(git(t, m, "ls-remote", "--heads", "origin", "refs/heads/"+m.BaseBranch)))
	if len(base) != 2 || base[0] != m.BaseRevision || base[1] != "refs/heads/"+m.BaseBranch {
		t.Fatal("prepare and verify the dedicated remote base before native execution")
	}
	shell(t, m.Root, "gh", "auth", "status", "--active", "--hostname", "github.com")
	dir := filepath.Join(m.Root, "s")
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("live state directory must be fresh")
	}
	started := invoke(t, m, dir, []string{"service", "start"}, nil)
	service := started.Data.(map[string]any)
	instance := service["instance_id"].(string)
	t.Cleanup(func() {
		req := request("cleanup-service", map[string]any{"expected_instance_id": instance})
		if _, err := command(m, dir, []string{"service", "stop", "--input", "-"}, &req); err != nil {
			t.Error(err)
			return
		}
		for until := time.Now().Add(30 * time.Second); time.Now().Before(until); time.Sleep(time.Second) {
			response, err := command(m, dir, []string{"service", "status"}, nil)
			if err == nil {
				raw, _ := json.Marshal(response.Data)
				var status struct {
					Status string `json:"status"`
				}
				_ = json.Unmarshal(raw, &status)
				if status.Status == "not_running" {
					r.ServiceStopped = true
					return
				}
			}
		}
		t.Error("service exit was not confirmed")
	})
	// Resolve the original submission by request ID if its CLI response was lost.
	startedAt := time.Now().UTC().Add(-time.Minute)
	t.Cleanup(func() {
		if r.RunID == "" {
			response, err := command(m, dir, []string{"inspect", "--request-id", "smoke-submit"}, nil)
			if err != nil {
				t.Error("original submission cleanup unresolved:", err)
				return
			}
			raw, _ := json.Marshal(response.Data)
			var data struct {
				Run *contracts.RunView `json:"run"`
			}
			_ = json.Unmarshal(raw, &data)
			if data.Run == nil || data.Run.RunID == "" {
				t.Error("original accepted run could not be recovered")
				return
			}
			r.RunID = data.Run.RunID
		}
		view, err := inspect(m, dir, r.RunID, false)
		if err != nil {
			t.Error(err)
			return
		}
		if m.Execution.Adapter == "orca-terminal" {
			body, diagErr := remoteDiagnostic(m, view)
			if diagErr == nil {
				diagErr = os.WriteFile(filepath.Join(*evidenceFlag, "remote-terminal-before-stop.json"), body, 0600)
			}
			if diagErr != nil {
				t.Logf("original remote diagnostic unavailable: %v", diagErr)
			}
		}
		if err = revokeOriginalRun(m, dir, r.RunID, "cleanup-run"); err != nil {
			t.Error(err)
			return
		}
		for until := time.Now().Add(60 * time.Second); time.Now().Before(until); time.Sleep(time.Second) {
			view, err = inspect(m, dir, r.RunID, false)
			if err != nil {
				t.Error(err)
				return
			}
			if view.StopStatus == "stopped" && !view.NewInputsAllowed {
				r.RunStopped = true
				save(t, "cleanup-run.json", view)
				if m.Execution.Adapter == "orca-terminal" {
					proof, proofErr := remoteStopEvidence(m, view)
					if proofErr != nil {
						t.Error("remote authentication cleanup unconfirmed", proofErr)
					} else {
						save(t, "authentication-cleanup.json", proof)
					}
				}
				if m.Execution.Adapter == "codex-app-server" && view.Execution.SessionID != "" {
					var cleanup struct {
						Revoked    bool      `json:"revocation_confirmed"`
						Repository string    `json:"repository"`
						Expires    time.Time `json:"expires_at"`
					}
					body, readErr := os.ReadFile(filepath.Join(dir, "runs", r.RunID, "app-server", "authentication-cleanup.json"))
					if readErr != nil || json.Unmarshal(body, &cleanup) != nil || !cleanup.Revoked || cleanup.Repository != livetest.Repository {
						t.Error("worker authentication cleanup is unverified", readErr)
					}
					if m.Level == "L2" {
						var settings contracts.CodexSettings
						if err := json.Unmarshal(m.Execution.Settings, &settings); err != nil {
							t.Error(err)
						}
						if settings.GitHubAuth == nil {
							if err := livetest.CheckBrokerCleanup(body); err != nil {
								t.Error(err)
							}
						}
					}
					save(t, "authentication-cleanup.json", cleanup)
				}
				return
			}
			if view.StopStatus == "unsupported" {
				break
			}
		}
		save(t, "cleanup-run.json", view)
		t.Errorf("original run stop not confirmed: %s %s", view.StopStatus, view.Diagnostic)
	})
	spec, _ := os.ReadFile(m.SpecPath)
	plan, _ := os.ReadFile(m.PlanPath)
	specText, planText := string(spec), string(plan)
	input := contracts.RunRequest{Title: "SteerRunner " + m.Level + " " + m.Scenario, Correlation: map[string]string{"fixture_id": m.ID, "scenario": m.Scenario}, Repository: contracts.Repository{URL: livetest.RemoteURL, LocalPath: m.RepositoryPath, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Kind: "inline", Path: "smoke.spec.md", Content: &specText}, Plan: contracts.Document{Kind: "inline", Path: "smoke.plan.md", Content: &planText}}, Workflow: contracts.Workflow{ID: "development", Version: 1}, Execution: m.Execution, Publication: m.Publication}
	if audited {
		input.Workflow = contracts.Workflow{ID: workflow.AuditedDevelopment, Version: 1, AuditModel: m.AuditModel}
	}
	// L2 exercises the production prompts and worker-owned Git/PR commands.
	// L3 keeps its explicit publisher gates for feedback and fault injection.
	if m.Level != "L2" {
		input.Workflow.PromptOverrides = smokePromptOverrides()
	}
	checkpoint("submit")
	req := request("smoke-submit", input)
	accepted := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
	r.RunID = accepted.Data.(map[string]any)["run_id"].(string)
	checkpoint("implementation")
	first := waitReady(t, m, dir, r.RunID, "")
	if m.Level == "L2" {
		common := strings.TrimSpace(string(shell(t, first.Execution.Workspace, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")))
		if !filepath.IsAbs(common) || !strings.EqualFold(filepath.Base(common), ".git") {
			t.Fatal("L2 requires the worker's actual Git common directory")
		}
		rel, err := filepath.Rel(filepath.Join(dir, "r"), filepath.Dir(common))
		if err != nil || len(rel) != 26 || strings.ContainsAny(rel, `/\`) {
			t.Fatal("L2 worker repository is outside the owned compact layout")
		}
		packed, err := os.ReadFile(filepath.Join(common, "packed-refs"))
		if err != nil || !bytes.Contains(packed, []byte("refs/")) {
			t.Fatal("L2 worker did not retain the packed reference precondition")
		}
		registered := string(shell(t, first.Execution.Workspace, "git", "worktree", "list", "--porcelain"))
		if strings.Count(registered, "worktree ") != 2 || strings.Count(registered, "branch refs/heads/codex/runner-") != 1 {
			t.Fatal("L2 Git common directory is shared with another worker")
		}
		save(t, "worker-git-repository.json", map[string]any{"common": common, "packed_refs": true, "worker_worktrees": 1, "production_prompts": true})
	}
	r.PRURL = first.PR.URL
	r.OriginalHead = first.PR.HeadRevision
	checkpoint("verify-original-pr")
	originalAt := verifyHead(t, m, first, startedAt)
	save(t, "original-pr-ready.json", first)
	if audited {
		auditWorkerSandbox(t, m, first)
		settings, modelErr := contracts.ExecutionModel(m.Execution)
		if modelErr != nil {
			t.Fatal(modelErr)
		}
		effort := settings.ReasoningEffort
		if effort == "" {
			effort = "medium"
		}
		if first.Audit == nil || len(first.Audit.Results) != 2 || first.Execution.RequestedModel != settings.Model || first.Execution.RequestedReasoningEffort != effort || first.Audit.Model != m.AuditModel.Model {
			t.Fatal("audit passes or execution model restoration are missing")
		}
		source := first.Audit.Results[0].SourceCommandID
		for i, step := range []string{workflow.StepAuditSpec, workflow.StepAuditCode} {
			a := first.Audit.Results[i]
			if a.StepID != step || a.Verdict != "pass" || a.Failure != "" || a.CommandID == "" || a.TurnID == "" || source == "" || a.SourceCommandID != source {
				t.Fatal("audit did not pass for one implementation report")
			}
		}
		if m.Execution.Adapter == "orca-terminal" {
			auditRemoteModels(t, m, first)
		}
		checkpoint("stop-and-cleanup")
		t.Logf("audited native run=%s PR=%s; leave test PR unmerged", r.RunID, r.PRURL)
		return
	}
	updated := first
	if m.Driver != "development-core" {
		feedback := "Regenerate only evidence/native-101.md using a newer current UTC timestamp than " + originalAt.Format(time.RFC3339Nano) + ". Follow the original Spec and Plan; run the checker and commit. Preserve the original branch, session and PR; publish through the original fixed publication command when authorized."
		if m.Level == "L2" {
			feedback = "Regenerate only evidence/native-101.md using a newer current UTC timestamp than " + originalAt.Format(time.RFC3339Nano) + ". Follow the original Spec and Plan and run the checker. Preserve the original branch, session and PR."
		}
		checkpoint("feedback")
		req = request("smoke-feedback", contracts.ReplyInput{ExpectedRevision: first.Revision, Kind: "feedback", Text: feedback})
		invoke(t, m, dir, []string{"reply", "--run-id", r.RunID, "--input", "-"}, &req)
		updated = waitReady(t, m, dir, r.RunID, first.PR.HeadRevision)
		if updated.PR.URL != first.PR.URL || updated.Execution.Branch != first.Execution.Branch || updated.Execution.SessionID != first.Execution.SessionID || updated.Execution.AttemptID != first.Execution.AttemptID {
			t.Fatal("feedback replaced original PR, branch, attempt or session")
		}
		r.FeedbackHead = updated.PR.HeadRevision
		checkpoint("verify-feedback-pr")
		verifyHead(t, m, updated, originalAt.Add(time.Nanosecond))
		save(t, "feedback-pr-ready.json", updated)
	}
	fresh, err := inspect(m, dir, r.RunID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := livetest.CheckPR(m, fresh); err != nil {
		t.Fatal(err)
	}
	if fresh.PR.HeadRevision != updated.PR.HeadRevision || fresh.PR.URL != updated.PR.URL {
		t.Fatal("PR changed before merge")
	}
	auditWorkerSandbox(t, m, fresh)
	checkpoint("merge-requested")
	shell(t, m.Root, "gh", "pr", "merge", r.PRURL, "--merge", "--match-head-commit", updated.PR.HeadRevision)
	merged, err := inspect(m, dir, r.RunID, true)
	if err != nil {
		t.Fatal(err)
	}
	if merged.PR == nil || merged.PR.QueryError != "" || !merged.PR.Merged || merged.PR.State != "merged" || merged.PR.MergeCommit == nil || merged.PR.MergedAt == nil || merged.PR.URL != r.PRURL || merged.PR.BaseBranch != m.BaseBranch || merged.PR.HeadRevision != updated.PR.HeadRevision {
		t.Fatal("Runner did not verify the actual merged PR")
	}
	r.MergeCommit = *merged.PR.MergeCommit
	checkpoint("verify-merged-content")
	save(t, "merged-run.json", merged)
	git(t, m, "fetch", "--no-tags", "origin", "+refs/heads/"+m.BaseBranch+":refs/remotes/origin/"+m.BaseBranch)
	if head := strings.TrimSpace(string(git(t, m, "rev-parse", "refs/remotes/origin/"+m.BaseBranch))); head != r.MergeCommit {
		t.Fatal("dedicated base does not contain the observed merge commit")
	}
	git(t, m, "merge-base", "--is-ancestor", updated.PR.HeadRevision, r.MergeCommit)
	mergedBody := git(t, m, "show", r.MergeCommit+":"+livetest.Artifact)
	headBody := git(t, m, "show", updated.PR.HeadRevision+":"+livetest.Artifact)
	if !bytes.Equal(mergedBody, headBody) {
		t.Fatal("merged artifact differs from verified PR head")
	}
	checkout := filepath.Join(m.Root, "merged")
	git(t, m, "worktree", "add", "--detach", checkout, r.MergeCommit)
	shell(t, checkout, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(checkout, "sr-smoke", "check.ps1"))
	checkpoint("stop-and-cleanup")
	t.Logf("%s %s run=%s PR=%s merge=%s", m.Level, m.Scenario, r.RunID, r.PRURL, r.MergeCommit)
}
