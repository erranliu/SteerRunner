//go:build windows && live

package native

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

// The real project's checker produces the report; evidence queries never ask
// the Agent to repeat work. This runs only in the existing host-local L2 Run.
func verifyNativeRunEvidence(t *testing.T, m livetest.Manifest, dir, id string) {
	t.Helper()
	before, err := inspect(m, dir, id, false)
	if err != nil {
		t.Fatal(err)
	}
	observe := func(clean bool) {
		response := invoke(t, m, dir, []string{"workspace-git", "--run-id", id}, nil)
		raw, _ := json.Marshal(response.Data)
		var git contracts.WorkspaceGit
		if json.Unmarshal(raw, &git) != nil || !git.Verified || git.Clean == nil || *git.Clean != clean || git.Commit == nil || *git.Commit != before.PR.HeadRevision {
			t.Fatal("real original workspace Git observation incorrect", response)
		}
	}
	observe(true)
	base := filepath.Join(before.Execution.Workspace, filepath.FromSlash(contracts.ArtifactDirectory))
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	relativeRoot := "native-r2-" + m.ID
	root := filepath.Join(base, relativeRoot)
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal("native evidence producer directory must be new", err)
	}
	path := filepath.Join(root, "actual-check.json")
	t.Cleanup(func() { _ = os.Remove(path); _ = os.Remove(root) })
	// The project may already own ignored reports under the shared artifact
	// root. Use a separate exclusive scratch file to prove Git dirt, retaining
	// every original report and the real checker bytes used for collection.
	dirtyPath := filepath.Join(before.Execution.Workspace, ".sr-native-git-"+m.ID+".json")
	dirty, err := os.OpenFile(dirtyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dirty.Close(); _ = os.Remove(dirtyPath) })
	data := shell(t, before.Execution.Workspace, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(before.Execution.Workspace, "sr-smoke", "check.ps1"))
	var report struct {
		Passed bool `json:"passed"`
	}
	if json.Unmarshal(data, &report) != nil || !report.Passed {
		t.Fatal("actual project checker did not produce a successful report")
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, writeErr := dirty.Write(data)
	closeErr := dirty.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal(writeErr, closeErr)
	}
	observe(false)
	discovered := invoke(t, m, dir, []string{"artifacts", "discover", "--run-id", id}, nil)
	input := request("native-r2-collect-"+m.ID, contracts.CollectArtifactInput{ExpectedRevision: before.Revision, RelativePath: relativeRoot + "/actual-check.json"})
	collected := invoke(t, m, dir, []string{"artifacts", "collect", "--run-id", id, "--input", "-"}, &input)
	artifact := collected.Data.(map[string]any)["artifact"].(map[string]any)["artifact_id"].(string)
	read := func() contracts.Response {
		response := invoke(t, m, dir, []string{"artifacts", "read", "--run-id", id, "--artifact-id", artifact}, nil)
		raw, _ := json.Marshal(response.Data)
		var value contracts.ArtifactContent
		if json.Unmarshal(raw, &value) != nil {
			t.Fatal("native artifact response undecodable")
		}
		decoded, err := base64.StdEncoding.DecodeString(value.ContentBase64)
		if err != nil || !bytes.Equal(data, decoded) || value.RunID != id {
			t.Fatal("real project report bytes changed", err)
		}
		return response
	}
	original := read()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dirtyPath); err != nil {
		t.Fatal(err)
	}
	replay := invoke(t, m, dir, []string{"artifacts", "collect", "--run-id", id, "--input", "-"}, &input)
	if !reflect.DeepEqual(collected, replay) || !reflect.DeepEqual(original, read()) {
		t.Fatal("deleted real report invalidated immutable collection or original request replay")
	}
	after, err := inspect(m, dir, id, false)
	if err != nil || !reflect.DeepEqual(before.Execution, after.Execution) || !reflect.DeepEqual(before.PR, after.PR) || before.Step != after.Step || before.Status != after.Status {
		t.Fatal("evidence queries changed original execution or PR", err)
	}
	observe(true)
	save(t, "r2-native-project-evidence.json", map[string]any{"producer": "original workspace sr-smoke/check.ps1", "discover": discovered, "collect": collected, "read": original, "after": after, "source_deleted_replay": true})
}
