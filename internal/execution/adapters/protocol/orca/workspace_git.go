package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
)

var workspaceGitOID = regexp.MustCompile("^(?:[0-9a-f]{40}|[0-9a-f]{64})$")

// PairedWorkspaceGit observes only the original workspace on its authenticated
// runtime. It does not create a terminal or invoke a Git process on this host.
func PairedWorkspaceGit(ctx context.Context, environment, host, runtime string, original Worktree) (string, bool, error) {
	if environment == "" || host == "" || runtime == "" {
		return "", false, errors.New("original paired Orca runtime is unavailable")
	}
	pairing, err := resolveStatePairing(environment)
	if err != nil {
		return "", false, err
	}
	currentHost, err := pairing.hostID()
	if err != nil || currentHost != host {
		return "", false, stateIdentityError("original workspace host changed")
	}
	conn, err := openStateSocketWithCapabilities(ctx, pairing, runtime, nil)
	if err != nil {
		return "", false, err
	}
	defer conn.close()
	return sampleWorkspaceGit(original, func(method string, params map[string]string) (json.RawMessage, error) {
		return callState(ctx, conn, method, params)
	})
}

type workspaceGitSample struct {
	Head, Branch string
	Entries      []json.RawMessage
}

func sampleWorkspaceGit(original Worktree, query func(string, map[string]string) (json.RawMessage, error)) (string, bool, error) {
	if original.ID == "" || original.InstanceID == "" || original.RepoID == "" || original.Path == "" || original.Branch == "" {
		return "", false, errors.New("original Orca worktree identity is incomplete")
	}
	selector := map[string]string{"worktree": "id:" + original.ID}
	verify := func() error {
		raw, err := query("worktree.show", selector)
		if err != nil {
			return err
		}
		var response WorktreeQuery
		if json.Unmarshal(raw, &response) != nil {
			return errors.New("invalid Orca worktree response")
		}
		w := response.Worktree
		if w.ID != original.ID || w.InstanceID != original.InstanceID || w.RepoID != original.RepoID ||
			w.Path != original.Path || w.Branch != original.Branch || w.BaseRef != original.BaseRef {
			return stateIdentityError("original Orca worktree changed during Git observation")
		}
		return nil
	}
	sample := func() (workspaceGitSample, error) {
		raw, err := query("git.status", selector)
		if err != nil {
			return workspaceGitSample{}, err
		}
		var s workspaceGitSample
		var flags struct{ Truncated, Limited bool }
		if json.Unmarshal(raw, &s) != nil || json.Unmarshal(raw, &flags) != nil || flags.Truncated || flags.Limited ||
			!workspaceGitOID.MatchString(s.Head) || s.Branch != original.Branch || s.Entries == nil {
			return s, errors.New("Orca Git response does not identify a complete original branch status")
		}
		return s, nil
	}
	if err := verify(); err != nil {
		return "", false, err
	}
	first, err := sample()
	if err != nil {
		return "", false, err
	}
	if err = verify(); err != nil {
		return "", false, err
	}
	second, err := sample()
	if err != nil {
		return "", false, err
	}
	if err = verify(); err != nil {
		return "", false, err
	}
	if first.Head != second.Head || len(first.Entries) != len(second.Entries) {
		return "", false, errors.New("workspace changed during Orca Git observation")
	}
	for i, entry := range first.Entries {
		if !bytes.Equal(entry, second.Entries[i]) {
			return "", false, errors.New("workspace changed during Orca Git observation")
		}
	}
	return second.Head, len(second.Entries) == 0, nil
}
