package codexworker

import (
	"fmt"
	"regexp"
	"strings"
)

// Contract identity retains the minimum version baseline for frozen resources.
const PolicyContract = "codex-policy-v10:0.159.2:explicit-access-mode:repository-auth-broker:never"
const PolicyMinimumCodexVersion = "0.159.2"
const PolicyCodexVersion = "codex-cli " + PolicyMinimumCodexVersion

var nativeVersionPattern = regexp.MustCompile(`^codex-cli (0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

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
	case "read_only":
		if host != "local" {
			return Permissions{}, fmt.Errorf("read_only is local only")
		}
		p.Sandbox, p.NetworkAccess = "read-only", false
	case "sandbox":
	case "full_access":
		p.Sandbox, p.WindowsSandbox = "danger-full-access", ""
	default:
		return Permissions{}, fmt.Errorf("unsupported access mode %q", mode)
	}
	return p, nil
}
func VerifyPolicyVersion(version string) error {
	version = strings.TrimSpace(version)
	parts := nativeVersionPattern.FindStringSubmatch(version)
	unsupported := func() error {
		return fmt.Errorf("native Codex launch requires codex-cli >= %s; observed %s", PolicyMinimumCodexVersion, version)
	}
	if parts == nil {
		return unsupported()
	}
	for _, identifier := range strings.Split(parts[4], ".") {
		if len(identifier) > 1 && identifier[0] == '0' && strings.Trim(identifier, "0123456789") == "" {
			return unsupported()
		}
	}
	for i, minimum := range strings.Split(PolicyMinimumCodexVersion, ".") {
		actual := parts[i+1]
		// Compare numeric components without overflowing on an untrusted version.
		if len(actual) > len(minimum) || len(actual) == len(minimum) && actual > minimum {
			return nil
		}
		if len(actual) < len(minimum) || actual < minimum {
			return unsupported()
		}
	}
	// A prerelease of the minimum version sorts before its stable release.
	if parts[4] != "" {
		return unsupported()
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
	if p.Sandbox == "read-only" {
		args = append(args, "--config", "features.apps=false", "--config", "features.plugins=false", "--config", "features.hooks=false", "--config", "features.multi_agent=false", "--config", "features.multi_agent_v2=false")
	}
	return args
}

func accessMode(sandbox string) string {
	if sandbox == "read-only" {
		return "read_only"
	}
	if sandbox == "danger-full-access" {
		return "full_access"
	}
	return "sandbox"
}
