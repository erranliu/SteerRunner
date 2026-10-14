//go:build windows

package host

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSRStartupIntent(t *testing.T) {
	dir, err := Normalize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	intent := []byte(`{"startup_id":"original-unresolved"}`)
	if err := os.WriteFile(bootstrapPath(dir), intent, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, failure := Status(ctx, dir)
	if failure != nil || info.Status != "startup_unverified" {
		t.Fatalf("unresolved startup hidden: %+v %v", info, failure)
	}
	_, failure = Ensure(ctx, dir, "must-not-launch.exe")
	if failure == nil || failure.Exit != 4 || failure.Detail.Code != "startup_unverified" {
		t.Fatalf("unresolved startup retried: %v", failure)
	}
	if _, err := os.Stat(filepath.Join(dir, "runner.db")); !os.IsNotExist(err) {
		t.Fatal("unresolved startup opened storage")
	}
	after, err := os.ReadFile(bootstrapPath(dir))
	if err != nil || string(after) != string(intent) {
		t.Fatal("original startup intent replaced")
	}
}
