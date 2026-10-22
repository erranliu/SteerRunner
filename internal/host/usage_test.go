package host

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestUsageAuthorizationFenceFailedStartup(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	policy := api.DefaultPolicy()
	policy.Enabled = false
	first := &contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{}, UsagePolicy: &policy}
	if err := savePendingStart(dir, first); err != nil {
		t.Fatal(err)
	}
	if err := savePendingStart(dir, nil); err != nil {
		t.Fatal(err)
	}
	saved, err := loadPendingStart(dir)
	if err != nil || saved.UsagePolicy != nil {
		t.Fatal("failed startup policy leaked into new default", saved, err)
	}
}
