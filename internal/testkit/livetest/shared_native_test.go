package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestNativeCandidateManifestIsolation(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tool := func(name string) Tool {
		p := filepath.Join(root, name)
		body := []byte(name)
		if err := os.WriteFile(p, body, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		return Tool{Path: p, SHA256: hex.EncodeToString(sum[:])}
	}
	runner, native, audit, spec, plan := tool("runner.exe"), tool("orca.exe"), tool("audit.exe"), tool("spec.md"), tool("plan.md")
	repo := filepath.Join(root, "SteerlineTest")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	settings := contracts.OrcaSettings{AccessMode: "full_access"}
	settings.Transport.Executable = audit.Path
	raw, _ := json.Marshal(settings)
	m := Manifest{Schema: "steerrunner-test/v1", ID: "012345abcdef", Level: "L3", Scenario: "orca-local.native-development", Driver: "native-capability-runner", Root: root, RunnerExe: runner.Path, RunnerSHA256: runner.SHA256, NativeCLI: &native, AuditTransport: &audit, SourceCommit: strings.Repeat("a", 40), SourceRevision: strings.Repeat("a", 40), BaseRevision: strings.Repeat("b", 40), BaseBranch: "codex/srtest/012345abcdef/base", RepositoryPath: repo, SpecPath: spec.Path, SpecSHA256: spec.SHA256, PlanPath: plan.Path, PlanSHA256: plan.SHA256, LocalRuntimeID: "original", LocalRepositoryPath: repo, Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: raw}, Publication: contracts.Publication{Repository: Repository, BaseBranch: "codex/srtest/012345abcdef/base", Settings: json.RawMessage(`{}`)}, Workflow: &contracts.Workflow{ID: "development", Version: 2}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := m.Validate(); err != nil {
		t.Fatal("valid native fixture rejected", err)
	}
	for _, name := range []string{"orca-local.codex", "orca-local.pi"} {
		t.Run(name, func(t *testing.T) {
			current := m
			current.NativeCLI, current.AuditTransport = nil, nil
			scenario, _, err := Lookup(name, "default")
			if err != nil {
				t.Fatal(err)
			}
			current.Scenario, current.Driver = name, scenario.Driver
			selected := settings
			if scenario.Agent == "pi" {
				selected.Agent = "pi"
			}

			current.Execution.Settings, _ = json.Marshal(selected)
			if err := current.Validate(); err != nil {
				t.Fatal("helper-free current manifest rejected", err)
			}
			selected.AgentLaunch.RuntimeExecutable = "C:/retired.exe"
			selected.AgentLaunch.RuntimeSHA256 = strings.Repeat("a", 64)
			current.Execution.Settings, _ = json.Marshal(selected)
			if current.Validate() == nil {
				t.Fatal("retired helper admitted")
			}
		})
	}
	t.Run("forward-slash-cli", func(t *testing.T) {
		portable := m
		cli := native
		cli.Path = filepath.ToSlash(cli.Path)
		portable.NativeCLI = &cli
		if err := portable.Validate(); err != nil {
			t.Fatal("canonical Windows slash variant rejected", err)
		}
	})
	for _, fault := range []string{"missing-cli", "missing-audit", "changed-cli", "changed-audit", "other-driver", "other-workflow", "helper-null", "helper-empty", "helper-upper"} {
		t.Run(fault, func(t *testing.T) {
			bad := m
			switch fault {
			case "missing-cli":
				bad.NativeCLI = nil
			case "missing-audit":
				bad.AuditTransport = nil
			case "changed-cli":
				changed := native
				changed.SHA256 = strings.Repeat("0", 64)
				bad.NativeCLI = &changed
			case "changed-audit":
				changed := audit
				changed.SHA256 = strings.Repeat("0", 64)
				bad.AuditTransport = &changed
			case "other-driver":
				bad.Driver = "development-feedback"
				bad.Scenario = "orca-local.codex"
			case "other-workflow":
				bad.Workflow = &contracts.Workflow{ID: "single-execution", Version: 1}
			default:
				var fields map[string]any
				_ = json.Unmarshal(raw, &fields)
				name, key, value := "agent_launch", "runtime_executable", any(nil)
				if fault == "helper-empty" {
					value = ""
				}
				if fault == "helper-upper" {
					name, key = "AGENT_LAUNCH", "RUNTIME_EXECUTABLE"
				}
				fields[name] = map[string]any{key: value}
				bad.Execution.Settings, _ = json.Marshal(fields)
			}
			if err := bad.Validate(); err == nil {
				t.Fatal("candidate fixture bypassed contract", fault)
			}
		})
	}
}
