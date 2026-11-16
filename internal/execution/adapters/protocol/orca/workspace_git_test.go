package orca

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPairedWorkspaceGitSamplesOriginalBranch(t *testing.T) {
	original := Worktree{ID: "registered", InstanceID: "original", RepoID: "repo", Path: "D:/worktrees/owned", Branch: "refs/heads/managed", BaseRef: "refs/heads/main"}
	head := strings.Repeat("a", 40)
	clean := `{"head":"` + head + `","branch":"refs/heads/managed","entries":[]}`
	dirty := `{"head":"` + head + `","branch":"refs/heads/managed","entries":[{"path":"new.txt","status":"untracked"}]}`
	for _, tc := range []struct {
		name, first, second string
		replaceAt           int
		wantClean, wantErr  bool
	}{
		{name: "clean", first: clean, second: clean, wantClean: true},
		{name: "untracked", first: dirty, second: dirty},
		{name: "dirty changed", first: dirty, second: strings.Replace(dirty, "new.txt", "other.txt", 1), wantErr: true},
		{name: "head changed", first: clean, second: strings.Replace(clean, head, strings.Repeat("b", 40), 1), wantErr: true},
		{name: "branch mismatch", first: strings.Replace(clean, "managed", "other", 1), second: clean, wantErr: true},
		{name: "missing inventory", first: strings.Replace(clean, "[]", "null", 1), second: clean, wantErr: true},
		{name: "truncated inventory", first: strings.Replace(clean, "[]", "[],\"truncated\":true", 1), second: clean, wantErr: true},
		{name: "worktree replaced", first: clean, second: clean, replaceAt: 3, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			shows, samples := 0, 0
			got, isClean, err := sampleWorkspaceGit(original, func(method string, params map[string]string) (json.RawMessage, error) {
				if !reflect.DeepEqual(params, map[string]string{"worktree": "id:" + original.ID}) {
					t.Fatalf("unbound request: %#v", params)
				}
				calls = append(calls, method)
				switch method {
				case "worktree.show":
					shows++
					w := original
					if shows == tc.replaceAt {
						w.InstanceID = "replacement"
					}
					return json.Marshal(WorktreeQuery{Worktree: w})
				case "git.status":
					samples++
					if samples == 1 {
						return json.RawMessage(tc.first), nil
					}
					return json.RawMessage(tc.second), nil
				default:
					return nil, errors.New("unexpected native mutation or query")
				}
			})
			if tc.wantErr {
				if err == nil || got != "" || isClean {
					t.Fatalf("ambiguous observation accepted: %q %v %v", got, isClean, err)
				}
				return
			}
			if err != nil || got != head || isClean != tc.wantClean {
				t.Fatalf("invalid observation: %q %v %v", got, isClean, err)
			}
			if !reflect.DeepEqual(calls, []string{"worktree.show", "git.status", "worktree.show", "git.status", "worktree.show"}) {
				t.Fatalf("samples not bracketed: %#v", calls)
			}
		})
	}
}
