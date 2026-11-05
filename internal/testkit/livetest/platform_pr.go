package livetest

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

type platformPRRef struct {
	Branch     string `json:"ref"`
	SHA        string `json:"sha"`
	Repository struct {
		Name string `json:"full_name"`
	} `json:"repo"`
}

type platformPR struct {
	URL         string     `json:"html_url"`
	Number      int        `json:"number"`
	State       string     `json:"state"`
	Merged      *bool      `json:"merged"`
	MergedAt    *time.Time `json:"merged_at"`
	MergeCommit string     `json:"merge_commit_sha"`
	Base, Head  platformPRRef
}

// CheckPlatformPR independently validates GitHub evidence against the Runner's
// published binding. Its result is the platform merge commit, if requested.
func CheckPlatformPR(raw []byte, bound *contracts.PRFact, merged bool) (string, error) {
	var pr platformPR
	if err := json.Unmarshal(raw, &pr); err != nil {
		return "", errors.New("invalid platform PR evidence")
	}
	if bound == nil || pr.URL != bound.URL || pr.Number != bound.Number || pr.Base.Repository.Name != bound.Repository ||
		pr.Base.Branch != bound.BaseBranch || pr.Head.Repository.Name != bound.HeadRepository || pr.Head.Branch != bound.HeadBranch ||
		pr.Head.SHA != bound.HeadRevision || !revisionPattern.MatchString(pr.Head.SHA) {
		return "", errors.New("platform PR differs from the original binding or head")
	}
	if pr.Merged == nil || *pr.Merged != merged {
		return "", errors.New("platform PR merge state differs from the expected state")
	}
	if !merged {
		if pr.State != "open" {
			return "", errors.New("platform PR is no longer open")
		}
		return "", nil
	}
	if pr.State != "closed" || pr.MergedAt == nil || pr.MergedAt.IsZero() || !revisionPattern.MatchString(pr.MergeCommit) {
		return "", errors.New("platform merge evidence is incomplete")
	}
	return pr.MergeCommit, nil
}
