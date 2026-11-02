//go:build windows

package integration

import (
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"
)

var controlledCasesFlag = flag.String("controlled-cases", "", "Exact parent/case selections, comma separated; other parents keep all cases")
var controlledCaseSelection map[string][]string
var controlledCaseResults = map[string][]controlledCaseResult{}

type controlledCaseResult struct {
	ID              string  `json:"id"`
	Passed          bool    `json:"passed"`
	DurationSeconds float64 `json:"duration_seconds"`
}

func knownControlledCases() map[string][]string {
	return map[string][]string{
		"TestSRResultFile":        {"optional-file-failure-business-loop", "final-seal"},
		"TestSRResultAcquisition": {"plain-marker", "audit-zero", "audit-positive", "audit-offline", "audit-report", "parser-unsupported", "parser-complete", "parser-tool"},
	}
}

func resolveControlledCases(value, legacyResultFile string) (map[string][]string, error) {
	known := knownControlledCases()
	wanted := map[string]map[string]bool{}
	add := func(parent, id string) error {
		found := false
		for _, candidate := range known[parent] {
			found = found || candidate == id
		}
		if !found || wanted[parent][id] {
			return fmt.Errorf("unknown or duplicate controlled case: %s/%s", parent, id)
		}
		if wanted[parent] == nil {
			wanted[parent] = map[string]bool{}
		}
		wanted[parent][id] = true
		return nil
	}
	if value != "" {
		for _, path := range strings.Split(value, ",") {
			parts := strings.Split(path, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("controlled case must be exact parent/case: %q", path)
			}
			if err := add(parts[0], parts[1]); err != nil {
				return nil, err
			}
		}
	}
	if legacyResultFile != "" {
		if wanted["TestSRResultFile"] != nil {
			return nil, fmt.Errorf("ResultFile cannot be selected through both legacy and controlled case flags")
		}
		for _, id := range strings.Split(legacyResultFile, ",") {
			if err := add("TestSRResultFile", id); err != nil {
				return nil, err
			}
		}
	}
	selected := map[string][]string{}
	for parent, ids := range wanted {
		for _, id := range known[parent] {
			if ids[id] {
				selected[parent] = append(selected[parent], id)
			}
		}
	}
	return selected, nil
}

func selectedControlledCases(parent string) []string {
	if selected, ok := controlledCaseSelection[parent]; ok {
		return selected
	}
	return knownControlledCases()[parent]
}

func runControlledCase(t *testing.T, parent, id string, run func(*testing.T)) {
	t.Helper()
	executed, skipped := false, false
	started := time.Now()
	passed := t.Run(id, func(t *testing.T) {
		executed = true
		defer func() { skipped = t.Skipped() }()
		run(t)
	})
	if !executed {
		t.Fatalf("required controlled case did not execute: %s/%s; use the per-parent case flag, not a global subtest filter", parent, id)
	}
	controlledCaseResults[parent] = append(controlledCaseResults[parent], controlledCaseResult{ID: id, Passed: passed && !skipped, DurationSeconds: time.Since(started).Seconds()})
}

func verifyControlledCaseExecution() error {
	for parent := range controlledCaseSelection {
		if len(controlledCaseResults[parent]) == 0 {
			return fmt.Errorf("explicit controlled parent did not execute: %s", parent)
		}
	}
	for parent, receipts := range controlledCaseResults {
		selected := selectedControlledCases(parent)
		seen := map[string]bool{}
		for _, receipt := range receipts {
			if seen[receipt.ID] || !receipt.Passed {
				return fmt.Errorf("duplicate, failed or skipped controlled case: %s/%s", parent, receipt.ID)
			}
			seen[receipt.ID] = true
		}
		if len(seen) != len(selected) {
			return fmt.Errorf("incomplete controlled cases: %s", parent)
		}
		for _, id := range selected {
			if !seen[id] {
				return fmt.Errorf("missing controlled case: %s/%s", parent, id)
			}
		}
	}
	return nil
}
