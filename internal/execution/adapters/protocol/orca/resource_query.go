package orca

import (
	"context"
	"encoding/json"
	"errors"
)

type Repository struct{ ID, Path, Kind string }

type RepositoryInventory struct {
	Repos     []Repository
	Truncated bool
	decoded   bool
}

func (r RepositoryInventory) Decoded() bool { return r.decoded }

// RepositoryListArgs lets callers retain their existing preparation queue,
// checkpoint and budget around the same native operation.
func RepositoryListArgs() []string { return []string{"repo", "list", "--json"} }

func DecodeRepositoryInventory(result Result) RepositoryInventory {
	var inventory RepositoryInventory
	err := json.Unmarshal(result.Payload, &inventory)
	inventory.decoded = err == nil
	return inventory
}

type Worktree struct {
	ID          string `json:"id"`
	InstanceID  string `json:"instanceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Head        string `json:"head"`
	Branch      string `json:"branch"`
	BaseRef     string `json:"baseRef"`
	DisplayName string `json:"displayName"`
}

type WorktreeQuery struct {
	Worktree Worktree
	decoded  bool
}

func (r WorktreeQuery) Decoded() bool { return r.decoded }

// ShowWorktree returns native parsing facts separately from invocation errors,
// so callers retain their existing identity/failure interpretation.
func (c Client) ShowWorktree(ctx context.Context, selector string) (Result, WorktreeQuery, error) {
	if selector == "" {
		return Result{}, WorktreeQuery{}, errors.New("Orca worktree show requires an explicit selector")
	}
	result, callErr := c.MetaCall(ctx, "worktree", "show", "--worktree", selector, "--json")
	var query WorktreeQuery
	err := json.Unmarshal(result.Payload, &query)
	query.decoded = err == nil
	return result, query, callErr
}
