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
	Provider         string `json:"provider_session,omitempty"`
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

func candidateState(m livetest.Manifest, client orca.Client, view contracts.RunView) (orca.StateResource, orca.AgentState, error) {
	runtime, err := m.OrcaRuntimeID()
	if err != nil {
		return orca.StateResource{}, orca.AgentState{}, err
	}
	prefix := "orca-terminal:" + runtime + ":"
	if !strings.HasPrefix(view.Execution.SessionID, prefix) {
		return orca.StateResource{}, orca.AgentState{}, errors.New("public session is not on the frozen runtime")
	}
	parts := strings.Split(strings.TrimPrefix(view.Execution.SessionID, prefix), ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return orca.StateResource{}, orca.AgentState{}, errors.New("public original terminal identity missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	terminal, err := client.InspectTerminal(ctx, orca.TerminalIdentity{RuntimeID: runtime, Handle: parts[0], IncarnationID: parts[1]})
	resource := orca.StateResource{RuntimeID: runtime, Handle: parts[0], IncarnationID: parts[1], WorktreeID: terminal.WorktreeID}
	if err != nil {
		return resource, orca.AgentState{}, err
	}
	if terminal.WorktreePath != view.Execution.Workspace || strings.TrimPrefix(terminal.Branch, "refs/heads/") != view.Execution.Branch || terminal.AgentIdentity != "codex" || terminal.WorktreeID == "" {
		return resource, orca.AgentState{}, errors.New("public workspace/agent differs from original terminal")
	}
	raw, err := orca.QueryState(ctx, m.Execution.Target.Environment, runtime, "id:"+resource.WorktreeID)
	if err != nil {
		return resource, orca.AgentState{}, err
	}
	initial, err := orca.DecodeOriginalState(raw, resource)
	if err != nil {
		return resource, initial, err
	}
	state, err := orca.QueryOriginalAgent(ctx, m.Execution.Target.Environment, resource, initial.PaneKey)
	return resource, state, err
}

func candidateCallAudit(m livetest.Manifest, handle string) (map[string]int, error) {
	f, err := os.Open(filepath.Join(m.Root, "evidence", "native-calls.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	counts := map[string]int{}
	requests := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 65536), 16<<20)
	for scanner.Scan() {
		var row struct {
			Phase string
			Argv  []string
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			return nil, errors.New("native call audit is incomplete")
		}
		if row.Phase != "dispatch" {
			continue
		}
		for _, arg := range row.Argv {
			if strings.Contains(arg, "--native-control") || strings.Contains(arg, "steerrunner-agent") || strings.Contains(arg, "--configuration") {
				return nil, errors.New("candidate invoked retired helper/control chain")
			}
		}
		if len(row.Argv) < 2 {
			return nil, errors.New("native audit operation missing")
		}
		operation := row.Argv[0] + " " + row.Argv[1]
		counts[operation]++
		if operation == "terminal send" {
			id := ""
			for i, arg := range row.Argv {
				if arg == "--retry-request" && i+1 < len(row.Argv) {
					id = row.Argv[i+1]
				}
			}
			if id == "" || requests[id] {
				return nil, errors.New("native input missing or repeated original request")
			}
			requests[id] = true
		}
		if operation == "terminal close" {
			for i, arg := range row.Argv {
				if arg == "--terminal" && i+1 < len(row.Argv) && row.Argv[i+1] == handle {
					counts["original close"]++
				}
			}
		}
	}
	return counts, scanner.Err()
}

func sharedNativeRunner(t *testing.T, m livetest.Manifest) {
	active := strings.HasSuffix(m.Scenario, "native-active-stop")
	r := sharedNativeResult{Scenario: m.Scenario, Level: m.Level, Stage: "preflight", Scope: "orca_terminal"}
	checkpoint := func() { save(t, "candidate-result.json", r); save(t, "scenario.json", r) }
	checkpoint()
	client := candidateClient(m)
	dir := filepath.Join(m.Root, "s")
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("candidate state directory must be fresh", err)
	}
	var original orca.StateResource
	var instance string
	submitted := false
	stopAttempted := false
	sendsAtRevoke := -1
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
			if err == nil && original.WorktreeID == "" && view.Execution.SessionID != "" {
				resource, _, resourceErr := candidateState(m, client, view)
				if resource.Handle != "" {
					original = resource
				}
				if resourceErr != nil {
					save(t, "cleanup-original-query.json", map[string]any{"resource": original, "error": resourceErr.Error()})
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
				if err == nil && !view.NewInputsAllowed && view.StopStatus == "stopped" && view.StopEvidenceScope == "orca_terminal" {
					r.RunStopped = true
					save(t, "online-stopped.json", view)
					break
				}
			}
			if !r.RunStopped {
				t.Error("original candidate Run stop is unverified", err)
			}
			if original.WorktreeID != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				inventory, inventoryErr := client.CompleteWorktreeInventory(ctx, orca.WorktreeInventoryScope{RuntimeID: original.RuntimeID, WorktreeID: original.WorktreeID})
				cancel()
				save(t, "original-inventory.json", map[string]any{"scope": original, "terminals": inventory, "error": fmt.Sprint(inventoryErr)})
				if inventoryErr != nil {
					t.Error(inventoryErr)
				}
				for _, terminal := range inventory {
					if !terminal.Exited() || terminal.Handle == original.Handle && terminal.IncarnationID != original.IncarnationID {
						t.Error("original workspace contains active or replacement resource", terminal)
					}
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
		counts, err := candidateCallAudit(m, original.Handle)
		save(t, "call-audit.json", map[string]any{"counts": counts, "error": fmt.Sprint(err), "sends_at_revocation": sendsAtRevoke})
		if err != nil || counts["original close"] != 1 || sendsAtRevoke >= 0 && counts["terminal send"] != sendsAtRevoke {
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
		if view.StopEvidenceScope != "orca_terminal" {
			t.Fatal("candidate frozen stop scope differs", view.StopEvidenceScope)
		}
		if view.Status == "blocked" || view.ResumeRequired {
			t.Fatal("candidate execution blocked", view.Phase, view.Diagnostic)
		}
		if active && view.Step != workflow.StepImplement {
			t.Fatal("activity window ended before original Started/working verification; stop this original Run")
		}
		if active && view.Execution.TurnStarted {
			resource, state, stateErr := candidateState(m, client, view)
			if stateErr == nil && state.MainAgent != nil && state.MainAgent.State == "working" && state.ProviderSession.ID != "" && state.MetadataObservation.ValidHookTransition() && state.TurnStartedAt == state.MainAgent.StateStartedAt {
				original = resource
				r.Provider = state.ProviderSession.ID
				r.ActiveVerified = true
				r.Stage = "active-stop"
				checkpoint()
				save(t, "active-original.json", map[string]any{"run": view, "resource": resource, "state": state, "metadata_observation": state.MetadataObservation})
				break
			}
		} else if !active && !r.FeedbackVerified && view.Status == "waiting_input" && view.Question != nil {
			resource, state, stateErr := candidateState(m, client, view)
			if stateErr != nil || state.ProviderSession.ID == "" {
				t.Fatal("original feedback session unavailable", stateErr)
			}
			original = resource
			firstSession = view.Execution.SessionID
			firstProvider = state.ProviderSession.ID
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
			resource, state, stateErr := candidateState(m, client, view)
			if stateErr != nil || resource != original || state.ProviderSession.ID != firstProvider {
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
	stop := request("b5-stop", contracts.RevisionInput{ExpectedRevision: view.Revision})
	stopAttempted = true
	response, err := command(m, dir, []string{"stop", "--run-id", r.RunID, "--input", "-"}, &stop)
	save(t, "online-stop-receipt.json", response)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := candidateCallAudit(m, original.Handle)
	if err != nil {
		t.Fatal(err)
	}
	sendsAtRevoke = counts["terminal send"]
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
