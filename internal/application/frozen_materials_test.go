package application

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestRecoveryFrozenMaterialsRejectReplacementWithoutRepairingOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.md")
	const original = "accepted frozen bytes"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runstate.Record{Materials: []runstate.Material{{MaterialView: contracts.MaterialView{Path: "spec.md", SnapshotPath: path, Digest: ra.Digest([]byte(original))}, Content: original}}}
	r.Request.Workflow.ID = "generate"
	before := canonical(r)
	if err := verifyFrozenMaterials(r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement content"), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyFrozenMaterials(r) == nil {
		t.Fatal("replacement frozen file was accepted")
	}
	if !bytes.Equal(before, canonical(r)) {
		t.Fatal("qualification rewrote accepted content or digest")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "replacement content" {
		t.Fatal("qualification silently repaired an external file", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if verifyFrozenMaterials(r) == nil {
		t.Fatal("absent snapshot was accepted")
	}
}
