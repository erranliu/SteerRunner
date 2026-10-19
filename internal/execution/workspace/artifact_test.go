package workspace

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"testing"
)

func TestR2ArtifactContainment(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(contracts.ArtifactDirectory))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../secret", "/secret", "C:/secret", "file:ads", "a\\b", "a/../b", ".", "a.", "a "} {
		if _, err := ReadArtifactSource(root, p); err == nil {
			t.Fatal("accepted unsafe path", p)
		}
	}
	if _, err := ReadArtifactSource(root, "missing"); err == nil {
		t.Fatal("missing reported success")
	}
	if err := os.WriteFile(filepath.Join(dir, "large"), make([]byte, contracts.MaxArtifactBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArtifactSource(root, "large"); err == nil {
		t.Fatal("accepted oversized file")
	}
	if err := os.Remove(filepath.Join(dir, "large")); err != nil {
		t.Fatal(err)
	}
	// Junctions do not require Windows symlink privileges. Outside links must
	// never be followed by discovery or collection, even when the root is valid.
	makeArtifactLink(t, filepath.Join(dir, "escape"), filepath.Dir(outside))
	if _, err := ReadArtifactSource(root, "escape/secret"); err == nil {
		t.Fatal("escaped collection root")
	}
	if _, err := DiscoverArtifacts(root); err == nil {
		t.Fatal("discovery followed a link")
	}
}
