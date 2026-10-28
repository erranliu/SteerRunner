package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func freezeProjectPolicies(ctx context.Context, r *runstate.Record) error {
	git := gitHost()
	return readProjectPolicies(r, func(args ...string) ([]byte, error) {
		return git.Output(ctx, r.RepositoryPath, append([]string{"--no-replace-objects"}, args...)...)
	})
}

// Read only exact entries and blobs from the accepted source commit, never
// the caller's checkout, current HEAD, execution worktree or alternate paths.
func readProjectPolicies(r *runstate.Record, output func(...string) ([]byte, error)) error {
	policies := &workflow.ProjectPolicies{Commit: r.Request.Repository.BaseRevision}
	for _, slot := range workflow.PolicySlots() {
		file := workflow.PolicyFile{PolicySlot: slot}
		raw, err := output("ls-tree", "-z", "--full-tree", policies.Commit, "--", ":(literal)"+slot.Path)
		if err != nil {
			return policyReadError(slot.Path, err)
		}
		if len(raw) != 0 {
			header, name, ok := strings.Cut(strings.TrimSuffix(string(raw), "\x00"), "\t")
			fields := strings.Fields(header)
			if !ok || name != slot.Path || len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" {
				return policyReadError(slot.Path, fmt.Errorf("regular repository file required"))
			}
			content, err := output("cat-file", "blob", fields[2])
			if err != nil {
				return policyReadError(slot.Path, err)
			}
			file.Exists, file.SHA256 = true, ra.Digest(content)
		}
		policies.Files = append(policies.Files, file)
	}
	// Publish all observations together only after every present file was read.
	r.Workflow.ProjectPolicies = policies
	return nil
}

func policyReadError(path string, err error) error {
	return contracts.Fail(2, "project_policy_unreadable", fmt.Sprintf("Cannot read project policy %s from the fixed source commit: %v", path, err))
}
