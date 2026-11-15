package application

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/evidence"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestGiteaFirstDraftMustMatchRemoteHead(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--bare", remote)
	git("init", "--initial-branch=candidate")
	git("commit", "--allow-empty", "-m", "Candidate")
	head := git("rev-parse", "HEAD")
	git("push", remote, "HEAD:refs/heads/candidate")

	engine := &Engine{ctx: context.Background()}
	r := &runstate.Record{RepositoryPath: root}
	r.Request.Repository.URL = filepath.ToSlash(remote)
	settings, err := json.Marshal(contracts.GiteaSettings{BaseURL: "http://localhost", Draft: true, DraftTitlePrefix: "WIP:"})
	if err != nil {
		t.Fatal(err)
	}
	r.Request.Publication = contracts.Publication{
		Provider: "gitea", Settings: settings,
	}
	r.Snapshot.Ref.WorkspaceBranch = "candidate"
	yes, no := true, false
	for _, test := range []struct {
		name string
		info evidence.PRInfo
		want bool
	}{
		{"verified", evidence.PRInfo{Draft: &yes, State: "open", HeadRevision: head}, true},
		{"ordinary", evidence.PRInfo{Draft: &no, State: "open", HeadRevision: head}, false},
		{"missing", evidence.PRInfo{State: "open", HeadRevision: head}, false},
		{"different head", evidence.PRInfo{Draft: &yes, State: "open", HeadRevision: strings.Repeat("a", len(head))}, false},
		{"closed", evidence.PRInfo{Draft: &yes, State: "closed_unmerged", HeadRevision: head}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if engine.initialDraftQualified(r, test.info) != test.want {
				t.Fatal("unexpected initial draft qualification")
			}
		})
	}
	r.PR = &contracts.PRFact{InitialDraftVerified: true}
	if !engine.initialDraftQualified(r, evidence.PRInfo{Draft: &no, State: "open", HeadRevision: head}) {
		t.Fatal("Ready transition invalidated the original draft proof")
	}
}
