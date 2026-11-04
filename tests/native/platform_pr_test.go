//go:build windows && live

package native

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func platformPR(t *testing.T, m livetest.Manifest, run contracts.RunView, merged bool) string {
	t.Helper()
	if err := livetest.CheckPR(m, run); err != nil {
		t.Fatal(err)
	}
	raw := shell(t, m.Root, "gh", "api", "--hostname", "github.com", "--method", "GET", fmt.Sprintf("repos/%s/pulls/%d", livetest.Repository, run.PR.Number))
	commit, err := livetest.CheckPlatformPR(raw, run.PR, merged)
	if err != nil {
		t.Fatal(err)
	}
	if merged {
		save(t, "merge-bound-run.json", run)
		save(t, "merge-platform.json", json.RawMessage(raw))
	}
	return commit
}
