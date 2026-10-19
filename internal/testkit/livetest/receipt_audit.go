package livetest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"strings"
)

// A title alone never exempts an extra terminal from business-resource counts.
// The frozen helper, exact readonly command, original ready receipt, full wire
// input, terminal incarnation, any returned native receipt and close agree.
type NativeReceiptAuditPolicy struct {
	Executable          string
	SHA256              string
	ConfigurationDigest string
	AccessMode          string
	Environment         string
}

func ReceiptAuditPolicy(m Manifest) NativeReceiptAuditPolicy {
	var s contracts.OrcaSettings
	if contracts.DecodeStrict(m.Execution.Settings, &s) != nil {
		return NativeReceiptAuditPolicy{}
	}
	a := s.AgentLaunch
	mode := s.AccessMode
	if mode == "" {
		mode = "sandbox"
	}
	config := codex.RemoteAgentConfig{Executable: a.Executable, Args: a.Args, Model: a.Model, ReasoningEffort: a.ReasoningEffort, AccessMode: mode, RepositoryURL: RemoteURL}
	if s.GitHubAuth != nil {
		auth := s.GitHubAuth
		config.GitHubAuth = &codex.AuthenticationConfig{AppID: auth.AppID, InstallationID: auth.InstallationID, PrivateKeyEnv: auth.PrivateKeyEnv}
	}
	return NativeReceiptAuditPolicy{Executable: a.RuntimeExecutable, SHA256: a.RuntimeSHA256, AccessMode: mode, Environment: m.Execution.Target.Environment, ConfigurationDigest: codex.RemoteConfigurationDigest(config)}
}

func auditArg(args []string, key string) string {
	value := ""
	for i, a := range args {
		if a == key && i+1 < len(args) {
			if value != "" {
				return ""
			}
			value = args[i+1]
		}
	}
	return value
}
func receiptHash(text string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) }
func sameAuditPath(a, b string) bool {
	return strings.EqualFold(strings.ReplaceAll(a, "\\", "/"), strings.ReplaceAll(b, "\\", "/"))
}
func auditJSON(c nativeAuditCall, runtime string, v any) bool {
	var e struct {
		OK   bool
		Meta struct {
			RuntimeID string `json:"runtimeId"`
		} `json:"_meta"`
		Result json.RawMessage
	}
	return !c.Failed && json.Unmarshal(c.Response, &e) == nil && e.OK && e.Meta.RuntimeID == runtime && json.Unmarshal(e.Result, v) == nil
}

func onlyReceiptCreateFlags(args []string, p NativeReceiptAuditPolicy) bool {
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		a := args[i]
		if seen[a] {
			return false
		}
		seen[a] = true
		switch a {
		case "--json":
		case "--worktree", "--title", "--command", "--environment":
			i++
			if i >= len(args) || args[i] == "" {
				return false
			}
		default:
			return false
		}
	}
	return seen["--json"] && seen["--worktree"] && seen["--title"] && seen["--command"] && auditArg(args, "--environment") == p.Environment
}

func onlyInitialTerminalFlags(args []string, p NativeReceiptAuditPolicy) bool {
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		flag := args[i]
		if seen[flag] {
			return false
		}
		seen[flag] = true
		switch flag {
		case "--json":
		case "--title", "--worktree", "--environment":
			i++
			if i >= len(args) || args[i] == "" {
				return false
			}
		default:
			return false
		}
	}
	return seen["--json"] && seen["--title"] && seen["--worktree"] && auditArg(args, "--environment") == p.Environment
}

func sameReceiptReady(a, b codex.RemoteReadiness) bool {
	return a.AuthenticationControl == b.AuthenticationControl && a.Verified == b.Verified && a.NativeTransport == b.NativeTransport && a.RuntimeSHA256 == b.RuntimeSHA256 && a.Contract == b.Contract && a.SessionID == b.SessionID && a.ConfigurationDigest == b.ConfigurationDigest && a.Repository == b.Repository && a.AccessMode == b.AccessMode && a.Process == b.Process && sameAuditPath(a.Workspace, b.Workspace) && a.CheckedAt.Equal(b.CheckedAt) && a.ExpiresAt.Equal(b.ExpiresAt)
}
