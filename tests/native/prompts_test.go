//go:build live

package native

import "github.com/erranliu/SteerRunner/internal/execution/workflow"

// Test orchestration belongs to the driver configuration, not frozen task
// materials. Authentication and permission preparation remain runtime-owned.
func smokePromptOverrides() map[string]string {
	return map[string]string{
		workflow.StepImplement: "Implement the frozen Spec and Plan, run the fixed checker and wait for its confirmed completion. If it reports an ordinary content-validation failure, correct only the specified artifact and rerun the unchanged checker to verify that correction. If a shell tool yields a session_id or running status, poll that same pending command; do not launch another checker while the original is pending or its result is unknown. The checker may contain a bounded driver-owned gate; preserve it and the original pending command. Commit only the specified artifact after the checker succeeds. Report actual changes and verification. Publication is a separate Runner stage.",
		"publish":              "Publish the already verified committed artifact by running powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sr-smoke/publish.ps1 once in the assigned sandbox. Wait for this original command to finish and report its actual PR URL. If the shell tool yields a session_id or running status, poll that same session until its final exit status and complete output; polling observes the original command and is required, not a second publication. Preserve an uncertain publication for original branch/PR reconciliation; never create a replacement. The test driver owns merge and test gates.",
	}
}
