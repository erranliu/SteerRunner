package codexworker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteCheckActorReplaysOriginalRequestAndRejectsConflict(t *testing.T) {
	setup := RemoteCheckSetup{CheckID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32), SourcePath: t.TempDir(), Input: contracts.CheckInput{Execution: contracts.Execution{Settings: json.RawMessage(`{"agent_launch":{"runtime_executable":"trusted"}}`)}}}
	request := RemoteCheckRequest{ID: strings.Repeat("c", 32), Method: "stop"}
	raw, _ := json.Marshal(request)
	line := base64.RawURLEncoding.EncodeToString(raw) + "\n"
	var out strings.Builder
	if err := ServeRemoteCheck(context.Background(), setup, strings.NewReader(line+line), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || lines[1] != lines[2] {
		t.Fatal("original response was not replayed", lines)
	}
	request.Method = "prepare"
	raw, _ = json.Marshal(request)
	conflict := base64.RawURLEncoding.EncodeToString(raw) + "\n"
	out.Reset()
	if err := ServeRemoteCheck(context.Background(), setup, strings.NewReader(line+conflict), &out); err == nil || !strings.Contains(err.Error(), "ID conflict") {
		t.Fatal("conflicting remote request was not rejected", err)
	}
}

func TestRemoteRuntimeMustStayOutsideWorkspaceAndGitWriteRoots(t *testing.T) {
	root := t.TempDir()
	wd := filepath.Join(root, "worker")
	git := filepath.Join(root, "git-objects")
	if err := remoteTrustedPaths(wd, []string{git}, []string{filepath.Join(root, "runtime", "agent.exe")}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{filepath.Join(wd, "agent.exe"), filepath.Join(git, "receipt.json"), wd} {
		if err := remoteTrustedPaths(wd, []string{git}, []string{bad}); err == nil {
			t.Fatal("mutable runtime path admitted", bad)
		}
	}
}

func TestRemoteCheckSetupWireRoundTripKeepsDeadlines(t *testing.T) {
	setup := RemoteCheckSetup{CheckID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32), SourcePath: t.TempDir(), ExecutionDeadline: time.Now().UTC().Add(time.Minute), CleanupDeadline: time.Now().UTC().Add(2 * time.Minute), Input: contracts.CheckInput{Execution: contracts.Execution{Settings: json.RawMessage(`{"agent_launch":{"runtime_executable":"trusted"}}`)}}}
	raw, err := json.Marshal(setup)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RemoteCheckSetup
	if err = contracts.DecodeStrict(raw, &decoded); err != nil || !decoded.ExecutionDeadline.Equal(setup.ExecutionDeadline) || !decoded.CleanupDeadline.Equal(setup.CleanupDeadline) {
		t.Fatal("actual remote setup wire format rejected", err)
	}
	var out strings.Builder
	if err = ServeRemoteCheck(context.Background(), decoded, strings.NewReader(""), &out); err != nil || !strings.Contains(out.String(), "SR_REMOTE_CHECK_ACTOR_READY") {
		t.Fatal("decoded setup did not reach ready", err)
	}
}

func TestRemoteCheckStartBeforePreparationReportsNoInput(t *testing.T) {
	setup := RemoteCheckSetup{CheckID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32), SourcePath: t.TempDir(), Input: contracts.CheckInput{Execution: contracts.Execution{Settings: json.RawMessage(`{"agent_launch":{"runtime_executable":"trusted"}}`)}}}
	raw, _ := json.Marshal(RemoteCheckRequest{ID: strings.Repeat("c", 32), Method: "start"})
	var output strings.Builder
	if err := ServeRemoteCheck(context.Background(), setup, strings.NewReader(base64.RawURLEncoding.EncodeToString(raw)+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(lines[1], RemoteCheckPrefix))
	if err != nil {
		t.Fatal(err)
	}
	var response RemoteCheckResponse
	if err = json.Unmarshal(raw, &response); err != nil || !response.InputNotSent || response.Turn != "" || response.Error != "original_session_missing" {
		t.Fatal("remote preflight rejection lost its certainty", response, err)
	}
}
