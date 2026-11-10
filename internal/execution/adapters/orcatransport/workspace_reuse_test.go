package orcatransport

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

func TestOrcaWorktreeReuseRebindsOnlyWorkspaceAndRequiresStop(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, host, s := workspaceFixtureOn(t, environment)
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			s, _, err = reliable.ApplySessionReceipt(s, nil, receipt, false)
			if err != nil {
				t.Fatal(err)
			}
			l.profileVersion = AgentSessionCloseProfileVersion
			l.setup.Snapshot = s
			state, err := l.agentState(s)
			if err != nil {
				t.Fatal(err)
			}
			next := s.Clone()
			next.Ref.RunID = "22222222222222222222222222222222"
			next.Ref.AttemptID = "33333333333333333333333333333333"
			next.Session.WorkflowRunID = next.Ref.RunID
			next.Session.ExecutionID = next.Ref.RunID
			next.Session.AttemptID = next.Ref.AttemptID
			next.ResourceGeneration = "new-generation"
			next.ConfigurationDigest = "new-config"
			next.Implementation = nil
			if err = l.reuseManagedWorkspace(context.Background(), &next); err == nil {
				t.Fatal("unconfirmed stop allowed reuse")
			}
			state.CloseAttempted = true
			state.CloseReceipt = json.RawMessage(`{"ok":true}`)
			s.Implementation, err = l.agentEnvelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			l.setup.Snapshot = s.Clone()
			before, _ := json.Marshal(s)
			calls := workspaceCalls(l, nil)
			if err = l.reuseManagedWorkspace(context.Background(), &next); err != nil {
				t.Fatal(err)
			}
			assertWorkspaceReadonly(t, *calls)
			fresh, err := readAgentSessionState(next, l.profile, false, next)
			after, _ := json.Marshal(s)
			if err != nil || fresh.SessionID == state.SessionID || fresh.Worktree != state.Worktree || fresh.CommandID != "" || fresh.Attempted || fresh.CloseAttempted || len(fresh.CloseReceipt) != 0 || len(fresh.Shells) != 0 || !bytes.Equal(after, before) {
				t.Fatal("old execution state crossed Run boundary", fresh, err)
			}
			host.workspace["instanceId"] = "replacement"
			if err = l.reuseManagedWorkspace(context.Background(), &next); err == nil {
				t.Fatal("replacement worktree accepted")
			}
		})
	}
}
