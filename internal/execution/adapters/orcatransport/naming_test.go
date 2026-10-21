package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestWorkspaceNamePrefixCreationAndRecovery(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		for _, prefix := range []string{"sr", "sl-103", "sl-103-c2", "acme-feature-7"} {
			t.Run(environment+"/"+prefix, func(t *testing.T) {
				l, _, s := fixtureOn(t, environment)
				s.Session.NamePrefix, s.Session.CorrelationID = prefix, "ignored-business-value"
				calls := workspaceCalls(l, nil)
				req := request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan})
				receipt, _, err := l.Execute(context.Background(), req)
				if err != nil || receipt.Disposition != b.OperationConfirmed {
					t.Fatal(receipt, err)
				}
				want := prefix + "-" + s.Session.WorkflowRunID
				creates := 0
				for _, args := range *calls {
					if args[0] == "worktree" && args[1] == "create" {
						creates++
						for i, arg := range args {
							if arg == "--name" && args[i+1] != want {
								t.Fatal("wrong creation name", args)
							}
						}
					}
				}
				var created workspaceCreation
				if json.Unmarshal(receipt.Operation.Receipt, &created) != nil || created.Worktree.DisplayName != want || creates != 1 {
					t.Fatal("creation and verified receipt disagree", created, creates)
				}
				req.Operation.Receipt, _ = json.Marshal(receipt)
				before := len(*calls)
				recovered, _, err := l.Reconcile(context.Background(), req)
				if err != nil || recovered.Disposition != b.OperationConfirmed {
					t.Fatal(recovered, err)
				}
				assertWorkspaceReadonly(t, (*calls)[before:])
				// A mismatched frozen prefix cannot adopt an existing creation receipt.
				req.Snapshot.Session.NamePrefix = "other"
				if _, _, err := l.Reconcile(context.Background(), req); err == nil {
					t.Fatal("foreign name accepted")
				}
				other := s
				other.Session.WorkflowRunID = strings.Repeat("f", 32)
				if workspaceName(other) == want {
					t.Fatal("different Runs shared a name")
				}
			})
		}
	}
}
