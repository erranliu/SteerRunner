//go:build windows

package integration

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSRNativeCatalogPublicCLI(t *testing.T) {
	testCount.Add(1)
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("APPDATA", root)
	t.Setenv("ORCA_USER_DATA_PATH", root)
	metadata := `{"version":1,"environments":[{"id":"one","name":"Machine","endpoints":[{"deviceToken":"secret-canary","endpoint":"ws://private"}]}]}`
	if err := os.WriteFile(filepath.Join(root, "orca-environments.json"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}
	response, exit, _, err := invoke(runnerEXE, []string{"native-catalog", "--input", "-", "--json"}, `{"kind":"environments"}`)
	if err != nil || exit != 0 || !response.OK {
		t.Fatal(response, exit, err)
	}
	body, _ := json.Marshal(response.Data)
	var result contracts.NativeCatalogResult
	if err = contracts.DecodeStrict(body, &result); err != nil || result.Contract != contracts.NativeCatalogContract || result.Source.ExecutableSHA256 == "" || result.Environments == nil || len(*result.Environments) != 1 || strings.Contains(string(body), "secret-canary") || strings.Contains(string(body), "ws://private") {
		t.Fatal(string(body), err)
	}
	for _, args := range [][]string{{"--state-dir", filepath.Join(root, "forbidden")}, {"--offline"}, {"--run-id", "run"}, {"--expected-instance-id", "instance"}} {
		argv := append([]string{"native-catalog", "--input", "-", "--json"}, args...)
		_, exit, _, err = invoke(runnerEXE, argv, `{"kind":"environments"}`)
		if err != nil || exit != 2 {
			t.Fatal(argv, exit, err)
		}
	}
	input, _ := json.Marshal(contracts.NativeCatalogRequest{Kind: "models", Adapter: "codex-app-server", Target: &contracts.Target{Kind: "local"}, Agent: "codex", Executable: helperEXE})
	response, exit, _, err = invoke(runnerEXE, []string{"native-catalog", "--input", "-", "--json"}, string(input))
	if err != nil || exit != 0 || !response.OK {
		t.Fatal(response, exit, err)
	}
	// Killing the CLI must close the non-inheritable Job owning its metadata child.
	trusted := contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}}
	trusted.Settings, _ = json.Marshal(map[string]string{"access_mode": "full_access", "executable": helperEXE})
	profileInput, _ := json.Marshal(contracts.NativeCatalogRequest{Kind: "models", Adapter: trusted.Adapter, Target: &trusted.Target, Agent: "codex", Execution: &trusted})
	response, exit, _, err = invoke(runnerEXE, []string{"native-catalog", "--input", "-", "--json"}, string(profileInput))
	if err != nil || exit != 0 || !response.OK {
		t.Fatal("trusted execution metadata", response, exit, err)
	}
	pidFile := filepath.Join(root, "metadata.pid")
	t.Setenv("STEERRUNNER_FIXTURE_METADATA_PID", pidFile)
	cmd := exec.Command(runnerEXE, "native-catalog", "--input", "-", "--json")
	cmd.Stdin = strings.NewReader(string(input))
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	var pid int
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
		if pid > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("metadata fixture did not start")
	}
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	state, err := windows.WaitForSingleObject(process, 5000)
	if err != nil || state != windows.WAIT_OBJECT_0 {
		t.Fatal("owned metadata process survived CLI", state, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatal("metadata command created state directory", entry.Name())
		}
	}
}
