//go:build windows

package integration

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

var runnerEXE string
var helperEXE string
var helperFlag = flag.String("helper-exe", "", "Native protocol fixture EXE")
var runnerFlag = flag.String("runner-exe", "", "Exact Runner EXE under test")
var evidenceFlag = flag.String("evidence-dir", "", "Owned directory for test evidence")
var baselineFlag = flag.String("baseline-exe", "", "Previous Runner EXE for v2 compatibility checks")
var testCount atomic.Int32
var evidenceMutex sync.Mutex

func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == "--native-inventory-fixture" {
		path := os.Args[2]
		args := os.Args[3:]
		if len(args) < 2 || args[0] != "terminal" || args[1] != "list" {
			os.Exit(2)
		}
		f, err := os.OpenFile(path+".calls", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(2)
		}
		raw, _ := json.Marshal(args)
		_, _ = f.Write(append(raw, '\n'))
		_ = f.Close()
		raw, err = os.ReadFile(path)
		if err != nil {
			os.Exit(2)
		}
		_, _ = os.Stdout.Write(raw)
		os.Exit(0)
	}
	if os.Getenv("STEERRUNNER_JOB_HELPER") != "" {
		os.Exit(runJobHelper())
	}
	flag.Parse()
	helperEXE = *helperFlag
	if *evidenceFlag == "" {
		*evidenceFlag = os.Getenv("STEERRUNNER_TEST_EVIDENCE_DIR")
	}
	var report *os.File
	if *evidenceFlag != "" {
		if err := os.MkdirAll(*evidenceFlag, 0700); err != nil {
			panic(err)
		}
		var err error
		report, err = os.Create(filepath.Join(*evidenceFlag, "integration.log"))
		if err != nil {
			panic(err)
		}
		os.Stdout, os.Stderr = report, report
	}
	runnerEXE = os.Getenv("STEERRUNNER_TEST_EXE")
	if *runnerFlag != "" {
		runnerEXE = *runnerFlag
	}
	root := ""
	if runnerEXE == "" {
		var err error
		root, err = os.MkdirTemp("", "steerrunner-test-build-")
		if err != nil {
			panic(err)
		}
		runnerEXE = filepath.Join(root, "steerrunner.exe")
		command := exec.Command("go", "build", "-trimpath", "-o", runnerEXE, "./cmd/steerrunner")
		command.Dir = filepath.Join("..", "..")
		if output, err := command.CombinedOutput(); err != nil {
			fmt.Fprintln(os.Stderr, string(output))
			os.RemoveAll(root)
			os.Exit(1)
		}
	}
	if helperEXE == "" {
		if root == "" {
			var err error
			root, err = os.MkdirTemp("", "steerrunner-test-helper-")
			if err != nil {
				panic(err)
			}
		}
		helperEXE = filepath.Join(root, "native-fixture.exe")
		command := exec.Command("go", "build", "-o", helperEXE, "./tests/helpers/native")
		command.Dir = filepath.Join("..", "..")
		if output, err := command.CombinedOutput(); err != nil {
			fmt.Fprintln(os.Stderr, string(output))
			os.Exit(1)
		}
	}
	code := m.Run()
	if code == 0 && testCount.Load() == 0 {
		fmt.Fprintln(os.Stderr, "No foundation scenario matched.")
		code = 1
	}
	if root != "" {
		os.RemoveAll(root)
	}
	if *evidenceFlag != "" {
		body, _ := json.Marshal(map[string]any{"exit_code": code, "scenario_count": testCount.Load(), "exe": runnerEXE, "finished_at": time.Now().UTC().Format(time.RFC3339Nano)})
		_ = os.WriteFile(filepath.Join(*evidenceFlag, "result.json"), body, 0600)
	}
	if report != nil {
		report.Close()
	}
	os.Exit(code)
}

func invoke(executable string, argv []string, input string) (contracts.Response, int, string, error) {
	var output, diagnostics bytes.Buffer
	command := exec.Command(executable, argv...)
	command.Stdin = strings.NewReader(input)
	command.Stdout, command.Stderr = &output, &diagnostics
	err := command.Run()
	exit := 0
	if failure, ok := err.(*exec.ExitError); ok {
		exit = failure.ExitCode()
	} else if err != nil {
		return contracts.Response{}, -1, diagnostics.String(), err
	}
	var response contracts.Response
	if *evidenceFlag != "" {
		evidenceMutex.Lock()
		file, fileErr := os.OpenFile(filepath.Join(*evidenceFlag, "caller-commands.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if fileErr == nil {
			_ = json.NewEncoder(file).Encode(map[string]any{"args": argv, "exit_code": exit, "stdout": output.String(), "stderr": diagnostics.String()})
			file.Close()
		}
		evidenceMutex.Unlock()
	}
	if decodeErr := contracts.DecodeStrict(output.Bytes(), &response); decodeErr != nil {
		return response, exit, diagnostics.String(), fmt.Errorf("invalid response: %s", output.Bytes())
	}
	return response, exit, diagnostics.String(), nil
}

func call(t *testing.T, dir string, argv []string, input string, expected int) contracts.Response {
	t.Helper()
	argv = append(argv, "--state-dir", dir, "--json")
	response, exit, diagnostics, err := invoke(runnerEXE, argv, input)
	if err != nil || exit != expected {
		body, _ := json.Marshal(response)
		t.Fatalf("%v exit=%d expected=%d error=%v response=%s stderr=%s", argv, exit, expected, err, body, diagnostics)
	}
	return response
}

func serviceInfo(t *testing.T, response contracts.Response) contracts.ServiceInfo {
	t.Helper()
	data, _ := json.Marshal(response.Data)
	var info contracts.ServiceInfo
	if err := contracts.DecodeStrict(data, &info); err != nil {
		t.Fatal(err)
	}
	return info
}

func stopRequest(id, instance string) string {
	body, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: id, Input: json.RawMessage(fmt.Sprintf(`{"expected_instance_id":%q}`, instance))})
	return string(body)
}

func cleanupService(t *testing.T, dir string, info contracts.ServiceInfo) {
	t.Helper()
	t.Cleanup(func() {
		response, _, _, _ := invoke(runnerEXE, []string{"service", "stop", "--state-dir", dir, "--input", "-", "--json"}, stopRequest("cleanup-"+info.InstanceID, info.InstanceID))
		_ = response
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			response, exit, _, err := invoke(runnerEXE, []string{"service", "status", "--state-dir", dir, "--json"}, "")
			if err == nil && exit == 0 {
				body, _ := json.Marshal(response.Data)
				var current contracts.ServiceInfo
				json.Unmarshal(body, &current)
				if current.Status == "not_running" {
					if err := waitOwnedService(info); err != nil {
						t.Errorf("owned service exit: %v", err)
					}
					return
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		if err := killOwnedService(info); err != nil {
			t.Errorf("cannot clean owned service: %v", err)
		}
	})
}

func TestSRStandalonePackage(t *testing.T) {
	testCount.Add(1)
	clean := t.TempDir()
	copyEXE := filepath.Join(clean, "steerrunner.exe")
	body, err := os.ReadFile(runnerEXE)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyEXE, body, 0700); err != nil {
		t.Fatal(err)
	}
	response, exit, _, err := invoke(copyEXE, []string{"version", "--json"}, "")
	if err != nil || exit != 0 || !response.OK {
		t.Fatalf("standalone version: %+v %v", response, err)
	}
	dir := filepath.Join(clean, "状态 directory")
	response, exit, _, err = invoke(copyEXE, []string{"service", "status", "--state-dir", dir, "--json"}, "")
	if err != nil || exit != 0 || serviceInfo(t, response).Status != "not_running" {
		t.Fatal("fresh directory status incorrect")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("read-only status created state")
	}
	response, exit, diagnostics, err := invoke(copyEXE, []string{"service", "start", "--state-dir", dir, "--json"}, "")
	if err != nil || exit != 0 {
		launchLog, _ := os.ReadFile(filepath.Join(dir, "logs", "launch.log"))
		serviceLog, _ := os.ReadFile(filepath.Join(dir, "logs", "service.log"))
		encoded, _ := json.Marshal(response)
		t.Fatalf("start: %s error=%v stderr=%s launch=%s service=%s", encoded, err, diagnostics, launchLog, serviceLog)
	}
	info := serviceInfo(t, response)
	cleanupService(t, dir, info)
	if !info.Detached || info.InstanceID == "" {
		t.Fatal("service not detached")
	}
	// The launcher has exited; the same service must still answer a new CLI.
	response, exit, _, err = invoke(copyEXE, []string{"capabilities", "--state-dir", dir, "--json"}, "")
	if err != nil || exit != 0 || !response.OK {
		t.Fatal("resident service lost after CLI exit")
	}
	data := response.Data.(map[string]any)
	if data["execution"].(map[string]any)["status"] != "implemented" {
		t.Fatal("execution capability is missing")
	}
	source := data["capability_source"].(map[string]any)
	if source["kind"] != "service" || source["instance_id"] != info.InstanceID || source["executable_sha256"] != info.ExecutableSHA256 || source["resource_digest"] != info.ResourceDigest {
		t.Fatal("online capabilities lost responding service identity", source)
	}
	onlineCatalog, _ := json.Marshal(data["capability_catalog"])
	response, exit, _, err = invoke(copyEXE, []string{"capabilities", "--offline", "--json"}, "")
	if err != nil || exit != 0 || !response.OK {
		t.Fatal("standalone offline capabilities failed", err)
	}
	offline := response.Data.(map[string]any)
	source = offline["capability_source"].(map[string]any)
	offlineCatalog, _ := json.Marshal(offline["capability_catalog"])
	if source["kind"] != "executable" || source["instance_id"] != nil || source["executable_sha256"] != info.ExecutableSHA256 || source["resource_digest"] != info.ResourceDigest || string(onlineCatalog) != string(offlineCatalog) {
		t.Fatal("offline build identity or static catalog changed", source)
	}
	request := `{"protocol":"steerrunner-cli/v1","request_id":"unsupported-submit","input":{"title":"中文","repository":{"url":"https://example.invalid/owner/repo.git","remote":"origin","base_branch":"main","base_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"documents":{"spec":{"kind":"inline","path":"spec.md","content":"Spec"},"plan":{"kind":"inline","path":"plan.md","content":"Plan"}},"workflow":{"id":"development","version":2},"execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"executable":"steerrunner-nonexistent-native.exe"}},"publication":{"provider":"gitea","repository":"owner/repo","base_branch":"main","settings":{"base_url":"https://example.invalid"}}}}`
	response, exit, _, err = invoke(copyEXE, []string{"submit", "--state-dir", dir, "--input", "-", "--json"}, request)
	if err != nil || exit != 4 || response.Error == nil || response.Error.Code != "environment_unavailable" {
		t.Fatal("unavailable native executable was not rejected before acceptance")
	}
	response, exit, _, err = invoke(copyEXE, []string{"inspect", "--state-dir", dir, "--request-id", "unsupported-submit", "--json"}, "")
	if err != nil || exit != 5 || response.Error == nil || response.Error.Code != "request_not_found" {
		t.Fatal("rejected submit persisted an accepted receipt")
	}
	for _, name := range []string{"runner.db", "logs/service.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSRServiceOwnership(t *testing.T) {
	testCount.Add(1)
	t.Run("offline_verification", testOfflineVerificationAfterWriterExitWithoutShutdownReceipt)
	t.Run("remote_offline_scope", testRemoteOfflineScope)
	t.Run("queries_do_not_start", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "runner")
		for _, args := range [][]string{{"capabilities"}, {"inspect", "--request-id", "missing"}, {"events", "--run-id", "missing"}} {
			response := call(t, dir, args, "", 4)
			if response.Error.Code != "service_unavailable" {
				t.Fatal("query did not report unavailable")
			}
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("query created state")
		}
	})
	t.Run("concurrent_alias", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "runner")
		aliases := []string{dir, strings.ToUpper(dir), filepath.Join(dir, ".")}
		var wait sync.WaitGroup
		responses := make([]contracts.Response, len(aliases))
		errors := make([]error, len(aliases))
		exits := make([]int, len(aliases))
		for i := range aliases {
			wait.Add(1)
			go func() {
				defer wait.Done()
				responses[i], exits[i], _, errors[i] = invoke(runnerEXE, []string{"service", "start", "--state-dir", aliases[i], "--json"}, "")
			}()
		}
		wait.Wait()
		for i := range aliases {
			if errors[i] != nil || exits[i] != 0 {
				t.Fatalf("concurrent start %d: exit=%d error=%v response=%+v", i, exits[i], errors[i], responses[i])
			}
		}
		info := serviceInfo(t, responses[0])
		cleanupService(t, dir, info)
		for _, response := range responses {
			if serviceInfo(t, response).InstanceID != info.InstanceID {
				t.Fatal("duplicate writer")
			}
		}
		other := filepath.Join(t.TempDir(), "runner")
		another := serviceInfo(t, call(t, other, []string{"service", "start"}, "", 0))
		cleanupService(t, other, another)
		if info.InstanceID == another.InstanceID {
			t.Fatal("directories share service")
		}
	})
	t.Run("parent_job", func(t *testing.T) { testParentJobs(t) })
	t.Run("different_release", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "runner")
		original := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
		cleanupService(t, dir, original)
		if len(original.ExecutableSHA256) != 64 {
			t.Fatal("service did not publish its executable identity")
		}
		body, err := os.ReadFile(runnerEXE)
		if err != nil {
			t.Fatal(err)
		}
		other := filepath.Join(t.TempDir(), "other-release.exe")
		// A PE overlay keeps the same version/protocol while changing the release hash.
		if err := os.WriteFile(other, append(body, []byte("different release")...), 0700); err != nil {
			t.Fatal(err)
		}
		response, exit, _, err := invoke(other, []string{"service", "start", "--state-dir", dir, "--json"}, "")
		if err != nil || exit != 5 || response.Error == nil || response.Error.Code != "directory_conflict" {
			t.Fatal("different release connected", exit, response, err)
		}
		if current := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0)); current.InstanceID != original.InstanceID {
			t.Fatal("release conflict replaced original writer")
		}
	})
	t.Run("stale_writer", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "runner")
		old := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
		cleanupService(t, dir, old)
		command := exec.Command(runnerEXE, "serve", "--state-dir", dir)
		output, err := command.CombinedOutput()
		failure, ok := err.(*exec.ExitError)
		if !ok || failure.ExitCode() != 5 {
			t.Fatalf("second writer not refused: %s %v", output, err)
		}
		if err := killOwnedService(old); err != nil {
			t.Fatal(err)
		}
		if err := waitOwnedService(old); err != nil {
			t.Fatal(err)
		}
		if info := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0)); info.Status != "not_running" {
			t.Fatal("dead writer reported ready")
		}
		current := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
		cleanupService(t, dir, current)
		if current.InstanceID == old.InstanceID {
			t.Fatal("service instance not renewed")
		}
		response := call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("stale-target", old.InstanceID), 5)
		if response.Error.Code != "stale_instance" {
			t.Fatal("stale target accepted")
		}
	})
	t.Run("shutdown_receipt", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "runner")
		old := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
		cleanupService(t, dir, old)
		input := stopRequest("stop-once", old.InstanceID)
		first := call(t, dir, []string{"service", "stop", "--input", "-"}, input, 0)
		for until := time.Now().Add(5 * time.Second); ; {
			status := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0))
			if status.Status == "not_running" {
				proof := status.LastShutdown
				if proof == nil || proof.InstanceID != old.InstanceID || proof.RequestID != "stop-once" || proof.ExecutableSHA256 != old.ExecutableSHA256 || proof.StateDir != old.StateDir || !proof.ResourcesSettled || proof.Diagnostic != "" {
					t.Fatalf("final original shutdown evidence missing: %+v", proof)
				}
				break
			}
			if time.Now().After(until) {
				t.Fatal("stop did not finish")
			}
			time.Sleep(30 * time.Millisecond)
		}
		current := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
		cleanupService(t, dir, current)
		query := call(t, dir, []string{"inspect", "--request-id", "stop-once"}, "", 0)
		command := query.Data.(map[string]any)["command"].(map[string]any)
		if command["action_status"] != "completed" {
			t.Fatal("persisted stop did not reconcile")
		}
		retry := call(t, dir, []string{"service", "stop", "--input", "-"}, input, 0)
		if retry.Data.(map[string]any)["command_id"] != first.Data.(map[string]any)["command_id"] {
			t.Fatal("retry created another command")
		}
		status := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0))
		if status.InstanceID != current.InstanceID || status.Status != "ready" {
			t.Fatal("retry stopped replacement instance")
		}
		conflict := call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("stop-once", current.InstanceID), 5)
		if conflict.Error.Code != "request_conflict" {
			t.Fatal("request conflict not preserved")
		}
		// A replacement that crashes cannot expose the original generation's
		// successful shutdown as proof for its own resources.
		if err := killOwnedService(current); err != nil {
			t.Fatal(err)
		}
		if err := waitOwnedService(current); err != nil {
			t.Fatal(err)
		}
		afterCrash := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0))
		if afterCrash.Status != "not_running" || afterCrash.LastShutdown != nil {
			t.Fatal("replacement crash reused an old shutdown proof", afterCrash)
		}
	})
}
