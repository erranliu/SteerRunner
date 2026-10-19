package orcatransport

import (
	"context"
	"testing"
)

func TestFixedWorkspaceAuditUsesFreshOriginalControlAndReconcilesPendingRead(t *testing.T) {
	for _, mode := range []string{"fresh", "lost-send", "workspace-foreign-branch", "workspace-dirty", "workspace-reader-live"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			c.mode = mode
			before := f.nativeSends
			fact, err := l.InspectLifecycleAuditWorkspace(context.Background(), s, "")
			if mode == "fresh" {
				if err != nil || !fact.Qualified {
					t.Fatal(fact, err)
				}
			} else if err == nil {
				t.Fatal("unverified source accepted", mode, fact)
			}
			if mode == "lost-send" {
				c.mode = "success"
				fact, err = l.InspectLifecycleAuditWorkspace(context.Background(), s, "")
				if err != nil || !fact.Qualified || c.sends != 1 {
					t.Fatal("pending original read was replaced", fact, err, c.sends)
				}
			}
			if mode == "fresh" || mode == "lost-send" {
				fact, err = l.InspectLifecycleAuditWorkspace(context.Background(), s, "")
				if err != nil || !fact.Qualified || c.sends != 2 {
					t.Fatal("new checkpoint reused stale source", fact, err, c.sends)
				}
			}
			if mode == "workspace-reader-live" {
				state, _ := l.state(s)
				if err = l.revokeAuthentication(context.Background(), state); err == nil || c.sends != 1 {
					t.Fatal("unsettled reader proof overwritten", err, c.sends)
				}
			}
			if c.creates != 1 || f.nativeSends != before {
				t.Fatal("source query created resources or business input", c.creates, f.nativeSends)
			}
		})
	}
}
