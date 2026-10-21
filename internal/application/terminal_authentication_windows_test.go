//go:build windows

package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
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
			t.Setenv("SR_TEST_TERMINAL_METADATA", string(canonical(orca.TerminalMetadata{Version: 3, Contract: terminaltools.Contract, RuntimeSHA256: digest, BundleSHA256: digest, PiSHA256: digest, DSHSHA256: digest})))
			// A private npm shim tests native path resolution without depending on
			// an installed Codex or executing any model, auth, or sandbox command.
			bin := t.TempDir()
			native := filepath.Join(bin, "node_modules", "@openai", "codex", "vendor", "x86_64-pc-windows-msvc", "bin", "codex.exe")
			if err := os.MkdirAll(filepath.Dir(native), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(native, []byte("MZ00"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "codex.cmd"), []byte(`node_modules\@openai\codex\bin\codex.js`), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			settings := contracts.OrcaSettings{Agent: agent}
			settings.Transport.Executable = git
			settings.AgentLaunch.Executable = agent
			settings.AgentLaunch.RuntimeExecutable = self
			f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
			f.input.Input = canonical(f.request)
			r := f.freeze()
			profile, err := orca.DecodeProfile(r.Profile[0].Payload)
			if err != nil || profile.Terminal == nil || profile.Terminal.AuthenticationState != state {
				t.Fatal("managed terminal repeated the Run ID in the socket root", profile, err)
			}
			c := profile.Terminal
			identity := codex.PolicyResourceIdentity{ProjectID: c.ProjectID, ExecutionID: c.ExecutionID, AttemptID: c.AttemptID, Generation: c.Generation, ConfigDigest: c.ConfigurationDigest}
			plan, err := codex.PlanAuthenticationTransport(c.AuthenticationState, r.Snapshot.Ref.WorkspacePath, c.RepositoryURL, identity)
			if err != nil {
				t.Fatal("frozen terminal authentication path exceeds the socket limit", err)
			}
			old := filepath.Join(state, "terminal-auth", r.ID)
			if _, err := codex.PlanAuthenticationTransport(old, plan.Workspace, plan.Remote, identity); err == nil {
				t.Fatal("fixture did not reproduce the old socket path failure")
			}
			for _, field := range []string{"run", "attempt", "generation"} {
				other := identity
				switch field {
				case "run":
					other.ExecutionID += "-other"
				case "attempt":
					other.AttemptID += "-other"
				case "generation":
					other.Generation += "-other"
				}
				isolated, err := codex.PlanAuthenticationTransport(state, plan.Workspace, plan.Remote, other)
				if err != nil || isolated.Directory == plan.Directory {
					t.Fatal("short transport path lost execution isolation", field, err)
				}
			}
		})
	}
}
