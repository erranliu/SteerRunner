package livetest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestCheckPlatformPRBindsOriginalHeadAndRequiresMergeEvidence(t *testing.T) {
	head, commit := strings.Repeat("a", 40), strings.Repeat("b", 40)
	bound := &contracts.PRFact{URL: "https://github.com/owner/repo/pull/17", Number: 17, Repository: "owner/repo", BaseBranch: "base", HeadRepository: "owner/repo", HeadBranch: "branch", HeadRevision: head}
	open, merged := false, true
	at := time.Now().UTC()
	fixture := platformPR{URL: bound.URL, Number: bound.Number, State: "closed", Merged: &merged, MergedAt: &at, MergeCommit: commit}
	fixture.Base.Branch, fixture.Base.Repository.Name = bound.BaseBranch, bound.Repository
	fixture.Head.Branch, fixture.Head.Repository.Name, fixture.Head.SHA = bound.HeadBranch, bound.HeadRepository, head
	for _, tc := range []struct {
		name           string
		change         func(*platformPR)
		merged, wantOK bool
	}{
		{"original merge", func(*platformPR) {}, true, true},
		{"original open", func(p *platformPR) { p.State, p.Merged = "open", &open }, false, true},
		{"foreign number", func(p *platformPR) { p.Number++ }, true, false},
		{"foreign head repository", func(p *platformPR) { p.Head.Repository.Name = "other/repo" }, true, false},
		{"changed head", func(p *platformPR) { p.Head.SHA = commit }, true, false},
		{"changed base", func(p *platformPR) { p.Base.Branch = "other" }, true, false},
		{"missing merge time", func(p *platformPR) { p.MergedAt = nil }, true, false},
		{"invalid merge commit", func(p *platformPR) { p.MergeCommit = "invalid" }, true, false},
		{"unknown merge state", func(p *platformPR) { p.Merged = nil }, true, false},
		{"closed without merge", func(p *platformPR) { p.Merged = &open }, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := fixture
			tc.change(&value)
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			got, err := CheckPlatformPR(raw, bound, tc.merged)
			if (err == nil) != tc.wantOK || (tc.wantOK && tc.merged && got != commit) {
				t.Fatalf("unexpected result: %q, %v", got, err)
			}
		})
	}
}
