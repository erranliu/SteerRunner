//go:build windows

package application

import (
	"context"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Freeze queries the selected helper's metadata, without starting an agent.
	if len(os.Args) == 2 && os.Args[1] == "--terminal-metadata" {
		if metadata := os.Getenv("SR_TEST_TERMINAL_METADATA"); metadata != "" {
			fmt.Print(metadata)
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestSRFreezeManagedTerminalAuthenticationPath(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		t.Run(agent, func(t *testing.T) {
			f := newBaseRefreshFixture(t, "origin", "main")
			state, err := os.MkdirTemp("", "sr-auth-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(state) })
			f.state = state
			self, _ := os.Executable()
			digest := strings.Repeat("a", 64)
			t.Setenv("SR_TEST_TERMINAL_METADATA", string(canonical(orca.TerminalMetadata{Version: 5, Contract: terminaltools.Contract, Authentication: "host-native", CodexPolicy: codex.PolicyContract, CodexTransport: codex.RemoteNativeTransport, RuntimeSHA256: digest, BundleSHA256: digest, PiSHA256: digest, DSHSHA256: digest})))
			// The managed terminal path must not discover or require Codex.
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			paths := []string{}
			for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
				if _, err := os.Stat(filepath.Join(directory, "codex.cmd")); err == nil {
					continue
				}
				if _, err := os.Stat(filepath.Join(directory, "codex.exe")); err == nil {
					continue
				}
				paths = append(paths, directory)
			}
			t.Setenv("PATH", strings.Join(paths, string(os.PathListSeparator)))
			if _, err := exec.LookPath("codex.cmd"); err == nil {
				t.Fatal("fixture still exposes Codex")
			}
			settings := contracts.OrcaSettings{AccessMode: "full_access", Agent: agent}
			settings.Transport.Executable = git
			settings.AgentLaunch.Executable = agent
			settings.AgentLaunch.RuntimeExecutable = self
			f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
			f.input.Input = canonical(f.request)
			r := f.freeze()
			profile, err := orca.DecodeProfile(r.Profile[0].Payload)
			if err != nil || profile.Terminal == nil || profile.Terminal.StateDirectory == "" {
				t.Fatal("managed terminal repeated the Run ID in the socket root", profile, err)
			}
			c := profile.Terminal
			if c.SandboxExecutable != "" || c.ReceiptVersion != 4 || r.Profile[0].CodecVersion != orca.FullAccessTerminalProfileVersion {
				t.Fatal("terminal kept sandbox executable or old contract", c, r.Profile[0].CodecVersion)
			}
			if strings.Contains(r.LaunchCommand, "authentication_state") {
				t.Fatal("serialized retired authentication state")
			}
		})
	}
}

func TestHostNativeAuthHelperMetadataBeforeLaunch(t *testing.T) {
	for _, kind := range []string{"current", "old-metadata", "old-transport", "broker", "wrong-build"} {
		t.Run(kind, func(t *testing.T) {
			f := newBaseRefreshFixture(t, "origin", "main")
			self, _ := os.Executable()
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			digest := strings.Repeat("a", 64)
			meta := orca.TerminalMetadata{Version: 5, Authentication: "host-native", CodexPolicy: codex.PolicyContract, CodexTransport: codex.RemoteNativeTransport, RuntimeSHA256: digest}
			switch kind {
			case "old-metadata":
				meta.Version = 4
			case "old-transport":
				meta.CodexTransport = "codex-remote-inline-v3"
			case "broker":
				meta.Authentication = "broker"
			case "wrong-build":
				meta.RuntimeSHA256 = strings.Repeat("b", 64)
			}
			t.Setenv("SR_TEST_TERMINAL_METADATA", string(canonical(meta)))
			settings := contracts.OrcaSettings{AccessMode: "full_access", Agent: "codex"}
			settings.Transport.Executable = git
			settings.AgentLaunch.Executable = self
			settings.AgentLaunch.RuntimeExecutable = self
			settings.AgentLaunch.RuntimeSHA256 = digest
			f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
			f.input.Input = canonical(f.request)
			record, err := freeze(context.Background(), f.state, f.input)
			if kind == "current" {
				if err != nil {
					t.Fatal(err)
				}
				profile, e := orca.DecodeProfile(record.Profile[0].Payload)
				if e != nil || profile.RuntimeSHA256 != digest {
					t.Fatal("helper identity was not frozen", e)
				}
				return
			}
			if err == nil {
				t.Fatal("old or mixed helper admitted")
			}
			for _, name := range []string{"runs", "workspaces"} {
				if _, e := os.Stat(filepath.Join(f.state, name)); !os.IsNotExist(e) {
					t.Fatal("rejected helper started execution resources", name, e)
				}
			}
		})
	}
}
