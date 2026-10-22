package orca

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Removes only the unused seed project registered by this acceptance batch.
// Orca CLI lacks repo removal; its existing authenticated repo.rm RPC is used.
func TestUsageLocalAcceptanceRepoCleanupLive(t *testing.T) {
	if os.Getenv("SR_USAGE_LOCAL_REPO_CLEANUP") != "1" {
		t.Skip("explicit owned local acceptance registration only")
	}
	root := os.Getenv("SR_USAGE_LOCAL_CLEANUP_ROOT")
	if !filepath.IsAbs(root) || !strings.HasPrefix(filepath.Base(root), "usage-local-") {
		t.Fatal("explicit owned acceptance root required")
	}
	raw, err := os.ReadFile(filepath.Join(root, "orca-repo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		OK     bool `json:"ok"`
		Result struct {
			Repo struct{ ID, Path string } `json:"repo"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &proof) != nil || !proof.OK || proof.Result.Repo.ID == "" || !strings.EqualFold(filepath.Clean(proof.Result.Repo.Path), filepath.Join(root, "seed")) {
		t.Fatal("original registration proof invalid")
	}
	exe := `C:\Users\lmas1\AppData\Local\Programs\orca\resources\bin\orca.exe`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cli := func(args ...string) Envelope {
		t.Helper()
		data, e := SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) { return exec.CommandContext(ctx, exe, args...).Output() })
		if e != nil {
			t.Fatal(e)
		}
		v, e := ParseEnvelope(data)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	status := cli("status", "--json")
	channel, err := OpenUsageFiles(ctx, "", status.Meta.RuntimeID, "local")
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	list := func() map[string]string {
		t.Helper()
		raw, e := callState(ctx, channel.socket, "repo.list", nil)
		if e != nil {
			t.Fatal(e)
		}
		var v struct {
			Repos []struct{ ID, Path string } `json:"repos"`
		}
		if json.Unmarshal(raw, &v) != nil || v.Repos == nil {
			t.Fatal("incomplete repository inventory")
		}
		out := map[string]string{}
		for _, r := range v.Repos {
			out[r.ID] = r.Path
		}
		return out
	}
	before := list()
	id := proof.Result.Repo.ID
	if !strings.EqualFold(filepath.Clean(before[id]), filepath.Clean(proof.Result.Repo.Path)) {
		t.Fatal("registered project changed")
	}
	listing := cli("worktree", "list", "--repo", "id:"+id, "--json")
	var worktrees struct {
		Worktrees []struct{ Path string } `json:"worktrees"`
		Truncated bool                    `json:"truncated"`
	}
	if json.Unmarshal(listing.Result, &worktrees) != nil || worktrees.Worktrees == nil || worktrees.Truncated {
		t.Fatal("incomplete worktree inventory")
	}
	for _, w := range worktrees.Worktrees {
		if !strings.EqualFold(filepath.Clean(w.Path), filepath.Join(root, "seed")) {
			t.Fatal("project has an unexpected workspace")
		}
	}
	terminals := cli("terminal", "list", "--worktree", "path:"+filepath.Join(root, "seed"), "--json")
	var terms struct {
		Terminals []json.RawMessage `json:"terminals"`
		Total     int               `json:"totalCount"`
		Truncated bool              `json:"truncated"`
	}
	if json.Unmarshal(terminals.Result, &terms) != nil || terms.Terminals == nil || len(terms.Terminals) != 0 || terms.Total != 0 || terms.Truncated {
		t.Fatal("seed project has terminals or incomplete inventory")
	}
	if _, err = callState(ctx, channel.socket, "repo.rm", map[string]any{"repo": "id:" + id}); err != nil {
		t.Fatal("removal outcome unconfirmed; do not replay", err)
	}
	after := list()
	delete(before, id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("project baseline not restored")
	}
	if _, err = os.Stat(filepath.Join(root, "seed", "README.md")); err != nil {
		t.Fatal("project removal altered fixture files", err)
	}
	result, _ := json.Marshal(map[string]any{"repo_id": id, "removed": true, "other_projects_unchanged": true, "fixture_files_preserved": true})
	if err = os.WriteFile(filepath.Join(root, "seed-registration-cleanup.json"), result, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("owned unused seed registration removed; other projects unchanged")
}
