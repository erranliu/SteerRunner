//go:build windows && live

package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

// Both requests enter the same real service before either is awaited. Product
// resource scheduling may serialize execution, but all identities remain owned
// by their original Run. Each test case still owns an independent service.
func developmentMultipleRuns(t *testing.T, m livetest.Manifest) {
	r := scenarioResult{Scenario: m.Scenario, Level: m.Level, Stage: "submit-two-runs"}
	var runs [2]contracts.RunView
	var ids [2]string
	var commands [2]string
	manifests := [2]livetest.Manifest{m, m}
	if m.PeerExecution != nil {
		manifests[1].Execution = *m.PeerExecution
	}
	t.Cleanup(func() {
		r.Passed = !t.Failed() && r.RunStopped && r.ServiceStopped && len(r.Runs) == 2
		if r.Passed {
			r.Stage = "completed"
		}
		save(t, "scenario.json", r)
	})
	dir := filepath.Join(m.Root, "s")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("multi-Run state must be new")
	}
	var service contracts.ServiceInfo
	response := invoke(t, m, dir, []string{"service", "start"}, nil)
	body, _ := json.Marshal(response.Data)
	if json.Unmarshal(body, &service) != nil || service.InstanceID == "" {
		t.Fatal("missing service identity")
	}
	t.Cleanup(func() {
		req := request("multi-service-stop", map[string]string{"expected_instance_id": service.InstanceID})
		if _, err := command(m, dir, []string{"service", "stop", "--input", "-"}, &req); err != nil {
			t.Error(err)
			return
		}
		for until := time.Now().Add(30 * time.Second); time.Now().Before(until); time.Sleep(time.Second) {
			res, err := command(m, dir, []string{"service", "status"}, nil)
			if err == nil && res.Data.(map[string]any)["status"] == "not_running" {
				r.ServiceStopped = true
				if m.Execution.Adapter == "orca-terminal" && m.Execution.Target.Kind == "orca_environment" {
					proof, err := verifyNativeStoppedScope(m, dir, ids[0])
					save(t, "offline-stopped.json", proof)
					if err != nil {
						r.RunStopped = false
						t.Error("offline multi-Run settlement unconfirmed", err)
						return
					}
					for _, id := range ids {
						found := false
						for _, item := range proof.Runs {
							if item.RunID == id && item.ResourcesSettled && item.StopEvidenceScope == "orca_terminal" {
								found = true
							}
						}
						if id == "" || !found {
							r.RunStopped = false
							t.Error("original native Run missing from settled scope", id)
						}
					}
				}
				return
			}
		}
		t.Error("multi-Run service exit unconfirmed")
	})
	t.Cleanup(func() {
		allStopped := true
		for i := range ids {
			if ids[i] == "" {
				res, err := command(m, dir, []string{"inspect", "--request-id", fmt.Sprintf("multi-submit-%d", i)}, nil)
				if err != nil {
					allStopped = false
					t.Error("submission outcome unresolved", err)
					continue
				}
				body, _ := json.Marshal(res.Data)
				var found struct {
					Run *contracts.RunView `json:"run"`
				}
				if json.Unmarshal(body, &found) != nil || found.Run == nil {
					allStopped = false
					t.Error("accepted multi-Run cannot be recovered")
					continue
				}
				ids[i] = found.Run.RunID
			}
			if err := revokeOriginalRun(m, dir, ids[i], fmt.Sprintf("multi-cleanup-%d", i)); err != nil {
				allStopped = false
				t.Error(err)
				continue
			}
			view := waitStopped(t, m, dir, ids[i])
			r.Runs = append(r.Runs, view)
			if view.StopStatus != "stopped" || view.NewInputsAllowed {
				allStopped = false
			}
			if manifests[i].Execution.Adapter == "orca-terminal" && manifests[i].Execution.Target.Kind == "orca_environment" {
				// Offline verification requires the service writer lease; the
				// earlier registered service cleanup runs after both Run stops.
				continue
			}
			if manifests[i].Execution.Adapter == "orca-terminal" && view.Execution.Agent == "codex" {
				proof, err := localStopEvidence(manifests[i], view)
				if err != nil {
					allStopped = false
					t.Error("remote multi-Run cleanup unconfirmed", err)
				} else {
					save(t, fmt.Sprintf("peer-%d-authentication-cleanup.json", i), proof)
				}
				continue
			}
			var fact struct {
				Revoked    bool   `json:"revocation_confirmed"`
				Repository string `json:"repository"`
			}
			if manifests[i].Execution.Adapter == "orca-terminal" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, "runs", ids[i], "app-server", "authentication-cleanup.json"))
			if err != nil || json.Unmarshal(data, &fact) != nil || !fact.Revoked || fact.Repository != livetest.Repository {
				allStopped = false
				t.Error("multi-Run capability cleanup unconfirmed", err)
			}
		}
		r.RunStopped = allStopped
	})
	started := time.Now().UTC().Add(-time.Minute)
	source, err := m.ExecutionRepositoryPath()
	if err != nil {
		t.Fatal(err)
	}
	for i := range ids {
		input := contracts.RunRequest{Title: fmt.Sprintf("SteerRunner multi-Run %d", i), Correlation: map[string]string{"fixture_id": m.ID, "peer": fmt.Sprint(i)}, Repository: contracts.Repository{URL: livetest.RemoteURL, LocalPath: source, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Path: "sr-smoke/spec.md"}, Plan: contracts.Document{Path: "sr-smoke/plan.md"}}, Workflow: contracts.Workflow{ID: "development", Version: 2, PromptOverrides: smokePromptOverrides()}, Execution: manifests[i].Execution, Publication: m.Publication}
		input.Workflow, err = manifests[i].RunWorkflow(smokePromptOverrides())
		if err != nil {
			t.Fatal(err)
		}
		req := request(fmt.Sprintf("multi-submit-%d", i), input)
		accepted := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
		ids[i] = accepted.Data.(map[string]any)["run_id"].(string)
		commands[i] = accepted.Data.(map[string]any)["command_id"].(string)
		for _, lookup := range [][]string{{"inspect", "--request-id", fmt.Sprintf("multi-submit-%d", i)}, {"inspect", "--command-id", commands[i]}} {
			found := invoke(t, m, dir, lookup, nil)
			body, _ := json.Marshal(found.Data.(map[string]any)["run"])
			var original contracts.RunView
			if json.Unmarshal(body, &original) != nil || original.RunID != ids[i] {
				t.Fatal("request/command lookup crossed Run identity")
			}
		}
		save(t, "multi-requests.json", ids)
	}
	if ids[0] == ids[1] || commands[0] == commands[1] {
		t.Fatal("distinct requests collapsed into one Run")
	}
	r.RunID = ids[0]
	for i := range ids {
		runs[i] = waitReady(t, manifests[i], dir, ids[i], "")
		verifyHead(t, manifests[i], runs[i], started)
		auditWorkerSandbox(t, manifests[i], runs[i])
		names := []string{"worker-sandbox-audit.json", "sandbox-readiness-audit.json"}
		if manifests[i].Execution.Adapter == "orca-terminal" {
			names = []string{"remote-worker-permission-audit.json"}
			if manifests[i].Execution.Target.Kind == "orca_environment" {
				names = []string{"native-call-audit.json", "native-launch-contract.json"}
			}
			if runs[i].Execution.Agent == "pi" || runs[i].Execution.Agent == "dsh" {
				names = []string{runs[i].Execution.Agent + "-native-permission-audit.json"}
			}
		}
		for _, name := range names {
			data, err := os.ReadFile(filepath.Join(*evidenceFlag, name))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(*evidenceFlag, fmt.Sprintf("peer-%d-%s", i, name)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		cursor := ""
		var all []contracts.Event
		for pages := 0; ; pages++ {
			if pages >= 100 {
				t.Fatal("event pagination did not terminate")
			}
			args := []string{"events", "--run-id", ids[i], "--limit", "100"}
			if cursor != "" {
				args = append(args, "--after", cursor)
			}
			page := invoke(t, m, dir, args, nil)
			body, _ := json.Marshal(page.Data)
			var events contracts.EventPage
			if json.Unmarshal(body, &events) != nil || len(events.Events) == 0 {
				t.Fatal("multi-Run events missing")
			}
			for _, event := range events.Events {
				if event.RunID != ids[i] {
					t.Fatal("foreign event in Run page")
				}
			}
			all = append(all, events.Events...)
			if !events.HasMore {
				break
			}
			if events.NextCursor == "" || events.NextCursor == cursor {
				t.Fatal("event cursor did not advance")
			}
			cursor = events.NextCursor
		}
		save(t, fmt.Sprintf("peer-%d-events.json", i), all)
		message := invoke(t, m, dir, []string{"last-message", "--run-id", ids[i]}, nil)
		body, _ := json.Marshal(message.Data)
		var latest contracts.LastMessageView
		if json.Unmarshal(body, &latest) != nil || latest.RunID != ids[i] || latest.Message != nil && (latest.Message.RunID != ids[i] || latest.Message.SessionID != runs[i].Execution.SessionID) {
			t.Fatal("last-message crossed native Run identity")
		}
		save(t, fmt.Sprintf("peer-%d-last-message.json", i), latest)
		if m.Execution.Target.Kind == "orca_environment" {
			assertNativeLastMessage(t, m, dir, runs[i], true)
		}
	}
	if runs[0].Execution.SessionID == runs[1].Execution.SessionID || runs[0].Execution.Workspace == runs[1].Execution.Workspace || runs[0].Execution.Branch == runs[1].Execution.Branch || runs[0].PR.URL == runs[1].PR.URL {
		t.Fatal("multi-Run session/workspace/branch/PR collision")
	}
	if err := revokeOriginalRun(m, dir, ids[0], "stop-first-peer"); err != nil {
		t.Fatal(err)
	}
	waitStopped(t, m, dir, ids[0])
	peer, err := inspect(m, dir, ids[1], true)
	if err != nil || !peer.NewInputsAllowed || peer.StopStatus == "stopped" || peer.Execution.SessionID != runs[1].Execution.SessionID || peer.PR == nil || peer.PR.URL != runs[1].PR.URL {
		t.Fatal("stopping first Run affected peer", err)
	}
	save(t, "peer-after-first-stop.json", peer)
	// Both workflows end at pr_ready. Recheck the original peer's native owner
	// and permissions after the first stop. Controlled isolation also holds a
	// peer active across revocation; no feedback transition is fabricated here.
	auditWorkerSandbox(t, manifests[1], peer)
	save(t, "peer-native-after-first-stop.json", peer)
	if peer.PR == nil || peer.PR.HeadRevision != runs[1].PR.HeadRevision || peer.Execution.TurnID != runs[1].Execution.TurnID {
		t.Fatal("stopping first Run changed peer completion source")
	}
	stopped, err := inspect(m, dir, ids[0], false)
	if err != nil || stopped.NewInputsAllowed || stopped.StopStatus != "stopped" || stopped.PR == nil || !strings.EqualFold(stopped.PR.HeadRevision, runs[0].PR.HeadRevision) {
		t.Fatal("peer observation changed the stopped Run", err)
	}
	r.PRURL = runs[0].PR.URL
	r.OriginalHead = runs[0].PR.HeadRevision
	r.Stage = "stop-and-cleanup"
	// PRs remain open as explicit test artifacts, as in the audited driver.
}

func waitStopped(t *testing.T, m livetest.Manifest, dir, id string) contracts.RunView {
	t.Helper()
	var view contracts.RunView
	for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		var err error
		view, err = inspect(m, dir, id, false)
		if err != nil {
			t.Error(err)
			return view
		}
		if view.StopStatus == "stopped" && !view.NewInputsAllowed {
			return view
		}
	}
	t.Error("original Run stop unconfirmed", id)
	return view
}
