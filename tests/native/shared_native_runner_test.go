//go:build windows && live

package native

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

type sharedNativeResult struct {
	Scenario         string `json:"scenario"`
	Level            string `json:"level"`
	Stage            string `json:"stage"`
	RunID            string `json:"run_id"`
	PRURL            string `json:"pr_url,omitempty"`
	Scope            string `json:"stop_evidence_scope"`
	Passed           bool   `json:"passed"`
	FeedbackVerified bool   `json:"feedback_verified"`
	ActiveVerified   bool   `json:"active_verified"`
	RunStopped       bool   `json:"run_stopped"`
	ServiceStopped   bool   `json:"service_stopped"`
	OfflineVerified  bool   `json:"offline_verified"`
	NoLegacyCalls    bool   `json:"no_legacy_calls"`
	Provider         string `json:"structured_session,omitempty"`
	Head             string `json:"head_revision,omitempty"`
}

func candidateClient(m livetest.Manifest) orca.Client {
	var settings contracts.OrcaSettings
	_ = contracts.DecodeStrict(m.Execution.Settings, &settings)
	return orca.New(orca.Config{CommandTimeout: "30s", Output: func(ctx context.Context, _ time.Duration, args ...string) ([]byte, error) {
		command := append(append([]string(nil), settings.Transport.Args...), args...)
		if m.Execution.Target.Environment != "" {
			command = append(command, "--environment", m.Execution.Target.Environment)
		}
		return orca.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
			return exec.CommandContext(ctx, settings.Transport.Executable, command...).Output()
		})
	}})
}

func candidateCallAudit(m livetest.Manifest, session string) (map[string]int, error) {
	raw, err := os.ReadFile(filepath.Join(m.Root, "evidence", "native-calls.jsonl"))
	if err != nil {
		return nil, err
	}
	// The CLI journal only proves absence of retired terminal writes. Structured
	// mutations bypass it and are never inferred from a zero CLI-send count.
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 65536), 16<<20)
	for scanner.Scan() {
		var row struct{ Argv, Args []string }
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			return nil, errors.New("invalid native CLI audit")
		}
		args := row.Argv
		if len(args) == 0 {
			args = row.Args
		}
		for _, arg := range args {
			if strings.Contains(arg, "--native-control") || strings.Contains(arg, "steerrunner-agent") {
				return nil, errors.New("retired helper invoked")
			}
		}
		if len(args) > 1 && args[0] == "terminal" && (args[1] == "send" || args[1] == "create") {
			return nil, errors.New("retired terminal writer invoked")
		}
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}
	counts := map[string]int{}
	if structuredPipe != nil {
		rows, err := structuredPipe.Calls(session)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			counts[row.Method]++
			if row.Method == "agentSession.create" || row.Method == "agentSession.send" {
				counts["business dispatch"]++
			}
		}
	}
	return counts, nil
}

func sharedNativeRunner(t *testing.T, m livetest.Manifest) {
	active := strings.HasSuffix(m.Scenario, "native-active-stop")
	r := sharedNativeResult{Scenario: m.Scenario, Level: m.Level, Stage: "preflight", Scope: "orca_agent_session"}
	checkpoint := func() { save(t, "candidate-result.json", r); save(t, "scenario.json", r) }
	checkpoint()
	dir := filepath.Join(m.Root, "s")
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("candidate state directory must be fresh", err)
	}
	var original livetest.StructuredRun
	var instance string
	submitted := false
	stopAttempted := false
	sendsAtRevoke := -1
	inputFingerprint := ""
	// This cleanup uses formal controls only, even after an earlier assertion
	// fails. Final success is written after original resources and writer exit.
	t.Cleanup(func() {
		if instance == "" {
			response, err := command(m, dir, []string{"service", "status"}, nil)
			var status contracts.ServiceInfo
			body, _ := json.Marshal(response.Data)
			_ = json.Unmarshal(body, &status)
			if err == nil && status.Status == "running" && status.StateDir == dir && status.ExecutableSHA256 == m.RunnerSHA256 {
				instance = status.InstanceID
			}
		}
		if submitted && r.RunID == "" {
			for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
				response, err := command(m, dir, []string{"inspect", "--request-id", "b5-submit"}, nil)
				if err != nil {
					break
				}
				var found struct {
					Run *contracts.RunView `json:"run"`
				}
				body, _ := json.Marshal(response.Data)
				_ = json.Unmarshal(body, &found)
				if found.Run != nil {
					r.RunID = found.Run.RunID
					break
				}
			}
		}
		if r.RunID != "" {
			view, err := inspect(m, dir, r.RunID)
			if err == nil && original.Binding.Worktree.ID == "" && view.Execution.SessionID != "" {
				_, resource, resourceErr := originalStructuredRecord(m, view)
				if resourceErr == nil {
					original = resource
				} else {
					save(t, "cleanup-original-query.json", map[string]any{"error": resourceErr.Error()})
				}
			}
			for until := time.Now().Add(2 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
				view, err = inspect(m, dir, r.RunID)
				if err == nil && view.StopStatus == "" && !stopAttempted {
					// A blocked Run can already forbid inputs while still owning
					// its original live terminal. That is not a stop receipt.
					stopAttempted = true
					response, stopErr := command(m, dir, []string{"stop", "--run-id", r.RunID, "--input", "-"}, ptrRequest(request("b5-stop", contracts.RevisionInput{ExpectedRevision: view.Revision})))
					save(t, "online-stop-receipt.json", response)
					if stopErr != nil {
						t.Error("original stop response unavailable; reconciliation remains required", stopErr)
					}
				}
				if err == nil && !view.NewInputsAllowed && view.StopStatus == "stopped" && view.StopEvidenceScope == "orca_agent_session" {
					r.RunStopped = true
					save(t, "online-stopped.json", view)
					break
				}
			}
			if !r.RunStopped {
				t.Error("original candidate Run stop is unverified", err)
			}
			if r.RunStopped {
				proof, proofErr := structuredStopEvidence(m, view)
				save(t, "original-session-stop.json", proof)
				if proofErr != nil {
					t.Error(proofErr)
				}
			}
		}
		if instance != "" {
			response, err := command(m, dir, []string{"service", "stop", "--input", "-"}, ptrRequest(request("b5-service-stop", map[string]any{"expected_instance_id": instance})))
			save(t, "service-stop-receipt.json", response)
			if err != nil {
				t.Error(err)
			}
			for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
				response, err = command(m, dir, []string{"service", "status"}, nil)
				if err == nil {
					var status struct{ Status string }
					body, _ := json.Marshal(response.Data)
					_ = json.Unmarshal(body, &status)
					if status.Status == "not_running" {
						r.ServiceStopped = true
						break
					}
				}
			}
		}
		if r.RunStopped && r.ServiceStopped {
			proof, err := verifyNativeStoppedScope(m, dir, r.RunID)
			save(t, "offline-stopped.json", proof)
			if err != nil {
				t.Error(err)
			} else {
				r.OfflineVerified = true
			}
		}
		if r.RunStopped && inputFingerprint != "" {
			stopped, inspectErr := inspect(m, dir, r.RunID)
			if inspectErr != nil {
				t.Error(inspectErr)
			} else if structuredInputFingerprint(t, m, stopped) != inputFingerprint {
				t.Error("new input after revocation")
			}
		}
		counts, err := candidateCallAudit(m, original.Binding.SessionID)
		save(t, "call-audit.json", map[string]any{"counts": counts, "error": fmt.Sprint(err), "dispatches_at_revocation": sendsAtRevoke, "wire_audit": structuredPipe != nil, "remote_proof": "original durable operations and complete history; not wire counts"})
		if err != nil || structuredPipe != nil && (counts["agentSession.create"] != 1 || counts["agentSession.close"] != 1 || sendsAtRevoke >= 0 && counts["business dispatch"] != sendsAtRevoke) {
			t.Error("original native calls or revocation fence unverified", counts, err)
		} else {
			r.NoLegacyCalls = true
		}
		r.Passed = !t.Failed() && r.RunStopped && r.ServiceStopped && r.OfflineVerified && r.NoLegacyCalls && ((active && r.ActiveVerified) || (!active && r.FeedbackVerified && r.PRURL != "" && r.Head != ""))
		if r.Passed {
			r.Stage = "completed"
		}
		checkpoint()
	})
	started, err := command(m, dir, []string{"service", "start"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var service struct {
		InstanceID string `json:"instance_id"`
	}
	body, _ := json.Marshal(started.Data)
	_ = json.Unmarshal(body, &service)
	instance = service.InstanceID
	if instance == "" {
		t.Fatal("original candidate service identity unavailable")
	}
	source, err := m.ExecutionRepositoryPath()
	if err != nil {
		t.Fatal(err)
	}
	selection, err := m.RunWorkflow(nil)
	if err != nil {
		t.Fatal(err)
	}
	if selection.PromptOverrides == nil {
		selection.PromptOverrides = map[string]string{}
	}
	if active {
		selection.PromptOverrides[workflow.StepImplement] = "This is an isolated activity-stop qualification. Run a native PowerShell tool command Start-Sleep -Seconds 120 and wait for it; do not modify files, commit, push or create a PR. The caller will revoke this original Run while the request is active. If the command returns before revocation, return exactly {\"outcome\":\"needs_input\",\"summary\":\"Activity window missed\",\"question\":\"Stop this qualification Run\"}; do not declare success or continue to publication."
	} else {
		selection.PromptOverrides[workflow.StepImplement] = "On the FIRST implementation input in this original session return exactly {\"outcome\":\"needs_input\",\"summary\":\"Awaiting native candidate feedback\",\"question\":\"Please authorize the frozen smoke implementation\"}; do not call tools or change files on that first input. On a later separate caller feedback in this same session implement the frozen Spec and Plan and run its real checker. Do not repeat this question after receiving feedback. Preserve the fixture tooling. Never merge a pull request."
	}
	input := contracts.RunRequest{Title: "Orca B5 " + m.Scenario, Correlation: map[string]string{"fixture_id": m.ID}, Repository: contracts.Repository{URL: livetest.RemoteURL, LocalPath: source, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Path: "sr-smoke/spec.md"}, Plan: contracts.Document{Path: "sr-smoke/plan.md"}}, Workflow: selection, Execution: m.Execution, Publication: m.Publication}
	submitted = true
	accepted, err := command(m, dir, []string{"submit", "--input", "-"}, ptrRequest(request("b5-submit", input)))
	save(t, "accepted.json", accepted)
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		RunID string `json:"run_id"`
	}
	body, _ = json.Marshal(accepted.Data)
	_ = json.Unmarshal(body, &receipt)
	r.RunID = receipt.RunID
	r.Stage = "executing"
	checkpoint()
	if r.RunID == "" {
		t.Fatal("formal candidate Run receipt missing")
	}
	firstSession, firstProvider := "", ""
	deadline := time.Now().Add(18 * time.Minute)
	for time.Now().Before(deadline) {
		view, err := inspect(m, dir, r.RunID)
		if err != nil {
			t.Fatal(err)
		}
		save(t, "latest-run.json", view)
		if view.StopEvidenceScope != "orca_agent_session" {
			t.Fatal("candidate frozen stop scope differs", view.StopEvidenceScope)
		}
		if view.Status == "blocked" || view.ResumeRequired {
			t.Fatal("candidate execution blocked", view.Phase, view.Diagnostic)
		}
		if active && view.Step != workflow.StepImplement {
			t.Fatal("activity window ended before original Started/working verification; stop this original Run")
		}
		if active && view.Execution.TurnStarted {
			resource, state, stateErr := structuredNativeState(m, view)
			if stateErr == nil && state.Tail.Working && state.CurrentTurn.State == "running" && state.CurrentTurn.StartedAt > 0 {
				original = resource
				r.Provider = resource.Binding.SessionID
				r.ActiveVerified = true
				r.Stage = "active-stop"
				checkpoint()
				save(t, "active-original.json", map[string]any{"run": view, "resource": resource, "state": state})
				break
			}
		} else if !active && !r.FeedbackVerified && view.Status == "waiting_input" && view.Question != nil {
			resource, _, stateErr := structuredNativeState(m, view)
			if stateErr != nil || resource.Binding.SessionID == "" {
				t.Fatal("original feedback session unavailable", stateErr)
			}
			original = resource
			firstSession = view.Execution.SessionID
			firstProvider = resource.Binding.SessionID
			feedback := request("b5-feedback", contracts.ReplyInput{ExpectedRevision: view.Revision, Kind: "feedback", Text: "Authorize implementation now in this same original session. Implement the frozen Spec and Plan, run the real checker, and complete the normal development workflow with commit/push and a test PR. Never merge the PR."})
			response, err := command(m, dir, []string{"reply", "--run-id", r.RunID, "--input", "-"}, &feedback)
			save(t, "pre-publication-feedback.json", response)
			if err != nil {
				t.Fatal(err)
			}
			r.FeedbackVerified = true
			r.Provider = firstProvider
			r.Stage = "development"
			checkpoint()
		} else if !active && view.Status == "pr_ready" {
			if !r.FeedbackVerified || view.Execution.SessionID != firstSession {
				t.Fatal("publication did not follow original pre-milestone feedback")
			}
			resource, _, stateErr := structuredNativeState(m, view)
			if stateErr != nil || resource.Binding.SessionID != original.Binding.SessionID || resource.Binding.RuntimeID != original.Binding.RuntimeID || resource.Binding.Worktree != original.Binding.Worktree || resource.Binding.SessionID != firstProvider {
				t.Fatal("feedback changed original terminal/provider", stateErr)
			}
			if err := livetest.CheckPR(m, view); err != nil {
				t.Fatal(err)
			}
			verifyCandidatePublication(t, m, view)
			r.PRURL = view.PR.URL
			r.Head = view.PR.HeadRevision
			r.Stage = "published-not-merged"
			checkpoint()
			save(t, "verified-pr-ready.json", view)
			break
		}
		time.Sleep(time.Second)
	}
	if active && !r.ActiveVerified || !active && r.PRURL == "" {
		t.Fatal("candidate did not reach its required business boundary")
	}
	view, err := inspect(m, dir, r.RunID)
	if err != nil {
		t.Fatal(err)
	}
	inputFingerprint = structuredInputFingerprint(t, m, view)
	stop := request("b5-stop", contracts.RevisionInput{ExpectedRevision: view.Revision})
	stopAttempted = true
	response, err := command(m, dir, []string{"stop", "--run-id", r.RunID, "--input", "-"}, &stop)
	save(t, "online-stop-receipt.json", response)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := candidateCallAudit(m, original.Binding.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	sendsAtRevoke = counts["business dispatch"]
}

func ptrRequest(value contracts.Request) *contracts.Request { return &value }

func verifyCandidatePublication(t *testing.T, m livetest.Manifest, view contracts.RunView) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, "gh", "api", fmt.Sprintf("repos/%s/pulls/%d", livetest.Repository, view.PR.Number)).Output()
	if err != nil {
		t.Fatal("independent test PR query failed", err)
	}
	var pr struct {
		HTMLURL string `json:"html_url"`
		Merged  bool
		State   string
		Head    struct {
			SHA, Ref string
			Repo     struct {
				FullName string `json:"full_name"`
			}
		}
		Base struct {
			SHA, Ref string
			Repo     struct {
				FullName string `json:"full_name"`
			}
		}
	}
	if json.Unmarshal(output, &pr) != nil || pr.Merged || pr.State != "open" || pr.HTMLURL != view.PR.URL || pr.Head.SHA != view.PR.HeadRevision || pr.Head.Ref != view.Execution.Branch || pr.Head.Repo.FullName != livetest.Repository || pr.Base.Repo.FullName != livetest.Repository || pr.Base.Ref != m.BaseBranch || pr.Base.SHA != m.BaseRevision {
		t.Fatal("independent PR repository/base/head identity changed")
	}
	save(t, "github-pr.json", json.RawMessage(output))
	// Reuse the existing independent Git-object/material/diff/checker
	// verification. It creates only a private QA checkout, never a worker PR.
	generatedAt := verifyHead(t, m, view, time.Now().Add(-time.Hour))
	save(t, "publication-check.json", map[string]any{"repository": livetest.Repository, "base": m.BaseRevision, "head": view.PR.HeadRevision, "branch": view.PR.HeadBranch, "artifact_generated_at": generatedAt, "pr": view.PR.URL, "merged": false})
}
