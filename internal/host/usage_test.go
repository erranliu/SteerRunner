package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestCostPolicyOptionalNullCannotErasePredicates(t *testing.T) {
	valid := `{"schema":"run-pricing-policy/v1","id":"fixture","version":"1","source":"synthetic test only","currency":"USD","enabled":true,"allow_completion_time":false,"rules":[{"provider":"fixture","model":"fixture","effective_from":"2020-01-01T00:00:00Z","effective_until":"2030-01-01T00:00:00Z","rates":{"input_uncached_per_million":"1","cache_read_per_million":"0","cache_write_per_million":"0","output_per_million":"2"}}]}`
	file := filepath.Join(t.TempDir(), "prices.json")
	check := func(raw string, wantValid bool) {
		t.Helper()
		if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := readPricingPolicy(file)
		if (err == nil) != wantValid {
			t.Fatal("policy loader and published schema disagree", wantValid, err)
		}
	}
	check(valid, true)
	for _, field := range []string{"tier", "min_context_tokens", "max_context_tokens"} {
		var document map[string]any
		if err := json.Unmarshal([]byte(valid), &document); err != nil {
			t.Fatal(err)
		}
		document["rules"].([]any)[0].(map[string]any)[field] = nil
		raw, _ := json.Marshal(document)
		check(string(raw), false)
	}
	check(strings.Replace(valid, `"provider":"fixture"`, `"min_context_tokens":0,"provider":"fixture"`, 1), true)
}

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
