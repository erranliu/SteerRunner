package livetest

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"reflect"
	"testing"
)

func TestFeedbackReadyRefreshesLateProviderHeadWithoutReexecuting(t *testing.T) {
	var old contracts.RunView
	if err := contracts.DecodeStrict([]byte(`{"run_id":"original","status":"pr_ready","execution":{"session_id":"session","attempt_id":"attempt","workspace":"workspace","branch":"branch"},"pr":{"url":"https://github.com/owner/repo/pull/1","provider":"command","repository":"owner/repo","number":1,"base_branch":"base","head_repository":"owner/repo","head_branch":"branch","head_revision":"old"}}`), &old); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"late-head", "already-fresh", "still-running", "initial", "unbound", "provider-error", "foreign-pr", "foreign-session"} {
		t.Run(mode, func(t *testing.T) {
			view := old
			cp := *old.PR
			view.PR = &cp
			head := "old"
			switch mode {
			case "already-fresh":
				view.PR.HeadRevision = "new"
			case "still-running":
				view.Status = "running"
			case "initial":
				head = ""
			case "unbound":
				view.PR = nil
			}
			flags := []bool{}
			providerError := errors.New("provider unavailable")
			got, err := ObservePublicationReady(head, func(refresh bool) (contracts.RunView, error) {
				flags = append(flags, refresh)
				if !refresh {
					return view, nil
				} // Default inspect deliberately retains old observed head.
				if mode == "provider-error" {
					return contracts.RunView{}, providerError
				}
				next := view
				p := *view.PR
				next.PR = &p
				next.PR.HeadRevision = "new"
				if mode == "foreign-pr" {
					next.PR.Number = 2
				}
				if mode == "foreign-session" {
					next.Execution.SessionID = "replacement"
				}
				return next, nil
			})
			refresh := mode != "already-fresh" && mode != "still-running" && mode != "initial" && mode != "unbound"
			want := []bool{false}
			if refresh {
				want = append(want, true)
			}
			if !reflect.DeepEqual(flags, want) {
				t.Fatal("unexpected read/mutation path", flags, want)
			}
			switch mode {
			case "provider-error":
				if !errors.Is(err, providerError) || got.PR.HeadRevision != "old" {
					t.Fatal("failed query promoted facts", got, err)
				}
			case "foreign-pr", "foreign-session":
				if err == nil || got.PR.HeadRevision != "old" {
					t.Fatal("changed identity admitted", got, err)
				}
			case "late-head":
				if err != nil || got.PR.HeadRevision != "new" {
					t.Fatal("late provider update remained stale", got, err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
