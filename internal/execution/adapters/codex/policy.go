package codexworker

import (
	"fmt"
	"strings"
)

const PolicyContract = "codex-policy-v10:0.159.2:explicit-access-mode:repository-auth-broker:never"
const PolicyCodexVersion = "codex-cli 0.159.2"

type Permissions struct {
	Sandbox           string `json:"sandbox"`
	WindowsSandbox    string `json:"windows_sandbox"`
	NetworkAccess     bool   `json:"network_access"`
	ApprovalPolicy    string `json:"approval_policy"`
	ApprovalsReviewer string `json:"approvals_reviewer"`
	Contract          string `json:"contract"`
}

// NativePermissions describes this adapter's single supported Windows launch.
// Other targets retain their own adapters and native admission checks.
func NativePermissions(goos, host, agent string, modes ...string) (Permissions, error) {
	if goos != "windows" || (host != "local" && host != "orca") || agent != "codex" {
		return Permissions{}, fmt.Errorf("fixed Codex Windows launch is unsupported for %s/%s/%s", goos, host, agent)
	}
	p := Permissions{Sandbox: "workspace-write", WindowsSandbox: "mxc", NetworkAccess: true, ApprovalPolicy: "never", ApprovalsReviewer: "user", Contract: PolicyContract}
	mode := "sandbox"
	if len(modes) > 1 {
		return Permissions{}, fmt.Errorf("exactly one access mode is allowed")
	}
	if len(modes) == 1 && modes[0] != "" {
		mode = modes[0]
	}
	switch mode {
	case "sandbox":
	case "full_access":
		p.Sandbox, p.WindowsSandbox = "danger-full-access", ""
	default:
		return Permissions{}, fmt.Errorf("unsupported access mode %q", mode)
	}
	return p, nil
}
func VerifyPolicyVersion(version string) error {
	if strings.TrimSpace(version) != PolicyCodexVersion {
		return fmt.Errorf("native Codex launch requires %s; observed %s", PolicyCodexVersion, strings.TrimSpace(version))
	}
	return nil
}
func (p Permissions) Args() []string {
	args := []string{"--config", "sandbox_mode=" + p.Sandbox,
		"--config", "allow_login_shell=false",
		"--config", "features.shell_snapshot=false", "--config", "features.shell_snapshot_v2=false",
		"--config", "approval_policy=" + p.ApprovalPolicy, "--config", "approvals_reviewer=" + p.ApprovalsReviewer,
		"--config", "sandbox_workspace_write.writable_roots=[]", "--config", "sandbox_workspace_write.exclude_tmpdir_env_var=true",
		"--config", "sandbox_workspace_write.exclude_slash_tmp=true", "--config", "sandbox_workspace_write.network_access=true"}
	if p.WindowsSandbox != "" {
		args = append(args[:2], append([]string{"--config", "windows.sandbox=" + p.WindowsSandbox}, args[2:]...)...)
	}
	return args
}

func accessMode(sandbox string) string {
	if sandbox == "danger-full-access" {
		return "full_access"
	}
	return "sandbox"
}
