//go:build windows

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSRExecutionConfigurationPublicCLI(t *testing.T) {
	testCount.Add(1)
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("APPDATA", root)
	input := `{"operation":"derive","execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"access_mode":"full_access","executable":"C:/not-installed/codex.exe","model":"selected"}},"workflow":{"id":"generate","version":1},"access_mode":"read_only"}`
	response, exit, _, err := invoke(runnerEXE, []string{"execution-config", "--input", "-", "--json"}, input)
	if err != nil || exit != 0 || !response.OK || response.RequestID != nil {
		t.Fatal(response, exit, err)
	}
	body, _ := json.Marshal(response.Data)
	var result contracts.ExecutionConfigurationResult
	if contracts.DecodeStrict(body, &result) != nil || result.Contract != contracts.ExecutionConfigurationContract || result.View.AccessMode != "read_only" || result.SourceIdentity == nil || result.Source.ExecutableSHA256 == "" || result.InputDigest != contracts.ConfigurationDigest(json.RawMessage(input)) {
		t.Fatal(string(body))
	}
	for _, raw := range []string{
		`{"operation":"describe","execution":{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"access_mode":"full_access","agent_launch":{"model":"chosen"}}},"workflow":{"id":"execute","version":1}}`,
		`{"operation":"compose","selection":{"adapter":"codex-app-server","target":{"kind":"local"},"agent":"codex","model":"chosen","access_mode":"full_access"},"workflow":{"id":"development","version":2}}`,
	} {
		response, exit, _, err = invoke(runnerEXE, []string{"execution-config", "--input", "-", "--json"}, raw)
		if err != nil || exit != 0 || !response.OK {
			t.Fatal(raw, response, exit, err)
		}
		body, _ = json.Marshal(response.Data)
		result = contracts.ExecutionConfigurationResult{}
		if contracts.DecodeStrict(body, &result) != nil || result.Identity == nil || result.SourceIdentity != nil {
			t.Fatal(string(body))
		}
	}
	for _, extra := range [][]string{{"--state-dir", filepath.Join(root, "forbidden")}, {"--offline"}, {"--run-id", "foreign"}, {"--expected-instance-id", "foreign"}} {
		args := append([]string{"execution-config", "--input", "-", "--json"}, extra...)
		response, exit, _, err = invoke(runnerEXE, args, input)
		if err != nil || exit != 2 || response.OK || response.Data != nil {
			t.Fatal(args, response, exit, err)
		}
	}
	t.Run("orca-preference-context", func(t *testing.T) {
		query := func(input any) contracts.ExecutionConfigurationResult {
			raw, _ := json.Marshal(input)
			response, exit, _, err := invoke(runnerEXE, []string{"execution-config", "--input", "-", "--json"}, string(raw))
			if err != nil || exit != 0 || !response.OK {
				t.Fatal(response, exit, err)
			}
			body, _ := json.Marshal(response.Data)
			var out contracts.ExecutionConfigurationResult
			if json.Unmarshal(body, &out) != nil {
				t.Fatal(string(body))
			}
			return out
		}
		base := contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"full_access"}`)}
		initial := query(contracts.ExecutionConfigurationRequest{Operation: "describe", Execution: &base, Workflow: contracts.Workflow{ID: "development", Version: 2}})
		model := "chosen"
		composed := query(contracts.ExecutionConfigurationRequest{Operation: "compose", BaseExecution: &base, Workflow: contracts.Workflow{ID: "development", Version: 2}, Selection: &contracts.ExecutionSelection{Adapter: base.Adapter, Target: base.Target, Agent: "codex", AccessMode: "full_access", Model: &model}})
		cleared := query(contracts.ExecutionConfigurationRequest{Operation: "compose", BaseExecution: composed.Execution, Workflow: contracts.Workflow{ID: "development", Version: 2}, Selection: &contracts.ExecutionSelection{Adapter: base.Adapter, Target: base.Target, Agent: "codex", AccessMode: "full_access"}, Unset: []string{"model"}})
		if contracts.ConfigurationDigest(initial.Identity.Context) != contracts.ConfigurationDigest(composed.Identity.Context) || contracts.ConfigurationDigest(initial.Identity.Context) != contracts.ConfigurationDigest(cleared.Identity.Context) {
			t.Fatal("public compose changed preference-only context")
		}
	})
	t.Run("stdin-deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, runnerEXE, "execution-config", "--input", "-", "--json")
		pipe, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer pipe.Close()
		var output bytes.Buffer
		cmd.Stdout = &output
		started := time.Now()
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		err = cmd.Wait()
		var response contracts.Response
		if err == nil || ctx.Err() != nil || json.Unmarshal(output.Bytes(), &response) != nil || response.Error == nil || response.Error.Code != "execution_configuration_timeout" || response.Data != nil || time.Since(started) < 29*time.Second {
			t.Fatal(err, ctx.Err(), output.String(), time.Since(started))
		}
	})
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("configuration command created state", entries, err)
	}
}
