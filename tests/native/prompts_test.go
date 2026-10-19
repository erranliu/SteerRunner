//go:build live

package native

import "github.com/erranliu/SteerRunner/internal/execution/workflow"

// Test orchestration belongs to the driver configuration, not frozen task
// materials. Authentication and permission preparation remain runtime-owned.
func smokePromptOverrides() map[string]string {
	return map[string]string{
		workflow.StepImplement: "Implement the frozen Spec and Plan, run the fixed checker once and wait for its completion. The checker may contain a bounded driver-owned gate; preserve it and the original pending command. Commit only the specified artifact after the checker succeeds. Report actual changes and verification. Publication is a separate Runner stage.",
		"publish":              "Publish the already verified committed artifact by running powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sr-smoke/publish.ps1 once in the assigned sandbox. Wait for this original command to finish and report its actual PR URL. Preserve an uncertain publication for original branch/PR reconciliation; never create a replacement. The test driver owns merge and test gates.",
	}
}
