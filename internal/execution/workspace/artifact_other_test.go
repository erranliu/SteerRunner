//go:build !windows

package workspace

import (
	"os"
	"testing"
)

func makeArtifactLink(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
