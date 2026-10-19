package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"testing"
	"time"
)

func TestOrcaCheckRepositoryUsesLocalPathWithoutChangingRemoteSelection(t *testing.T) {
	for _, name := range []string{"local-selected", "local-missing", "local-duplicate", "remote-ambiguous"} {
		t.Run(name, func(t *testing.T) {
			input := contracts.CheckInput{Repository: contracts.Repository{URL: "https://github.com/owner/repo.git", LocalPath: `C:\project`}, Execution: contracts.Execution{Target: contracts.Target{Kind: "local"}}}
			if name == "local-missing" {
				input.Repository.LocalPath = `C:\missing`
			}
			if name == "remote-ambiguous" {
				input.Execution.Target = contracts.Target{Kind: "orca_environment", Environment: "remote"}
			}
			s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32), Input: input}, settings: contracts.OrcaSettings{AgentLaunch: contracts.AgentSettings{RuntimeExecutable: "C:/trusted/agent.exe"}}}
			creations := 0
			s.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				result := map[string]any{}
				switch args[0] + " " + args[1] {
				case "repo list":
					other := "C:/other"
					if name == "local-duplicate" {
						other = "C:/project/"
					}
					rows := []any{}
					for i, path := range []string{"C:/project/", other} {
						rows = append(rows, map[string]any{"id": fmt.Sprintf("repo-%d", i), "path": path, "kind": "git", "gitRemoteIdentity": map[string]string{"remoteUrl": input.Repository.URL}})
					}
					result["repos"] = rows
				case "worktree show":
					if args[3] != "path:C:/project/" {
						t.Fatal("local project selector changed", args)
					}
					result["worktree"] = map[string]any{"id": "repo-0::project", "path": "C:/project/", "isMainWorktree": true}
				case "terminal create":
					creations++
					result["terminal"] = map[string]string{"handle": "actor", "incarnationId": "original", "worktreeId": "repo-0::project"}
				default:
					t.Fatal("unexpected check preparation call", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": "runtime"}})
			}})
			binding, err := s.Initialize(context.Background())
			if name == "local-selected" {
				if err != nil || creations != 1 || binding.Process == nil || binding.Process.RuntimeID != "runtime" {
					t.Fatal(binding, err, creations)
				}
			} else if err == nil || creations != 0 {
				t.Fatal("ambiguous/missing project created an actor", binding, err, creations)
			}
		})
	}
}

func TestRemoteActorControlRequiresFullLineAndOriginalResponseIdentity(t *testing.T) {
	for _, scenario := range []string{"normal", "changed-incarnation", "wrong-runtime", "partial-write", "lost-response", "lost-response-no-record", "read-failure-once", "show-failure-once", "prefixed-frame", "identity-read-once", "identity-changed-after-retry", "identity-unavailable", "permanent-read-failure", "wrong-response-id"} {
		t.Run(scenario, func(t *testing.T) {
			sends, shows, reads := 0, 0, 0
			var response codex.RemoteCheckResponse
			s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}, actorReady: true}
			s.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				runtime := "original"
				result := map[string]any{}
				switch args[1] {
				case "show":
					shows++
					if scenario == "show-failure-once" && shows == 1 {
						return nil, errors.New("native_cli_request_unverified")
					}
					if (scenario == "identity-read-once" || scenario == "identity-changed-after-retry") && shows == 2 || scenario == "identity-unavailable" && shows >= 2 {
						return nil, errors.New("native_cli_request_unverified")
					}
					incarnation := "incarnation"
					if scenario == "identity-changed-after-retry" && shows >= 3 {
						incarnation = "replacement"
					}
					if scenario == "changed-incarnation" && shows > 1 {
						incarnation = "replacement"
					}
					result["terminal"] = map[string]string{"handle": "actor", "incarnationId": incarnation}
				case "send":
					sends++
					payload := ""
					for i, arg := range args {
						if arg == "--enter" || arg == "--wait-submit" || arg == "--retry-request" {
							t.Fatal("machine actor entered model prompt tracking")
						}
						if arg == "--text" && i+1 < len(args) {
							payload = args[i+1]
						}
					}
					if !strings.HasSuffix(payload, "\r") {
						t.Fatal("incomplete actor line")
					}
					body, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(payload, "\r"))
					var request codex.RemoteCheckRequest
					if err != nil || json.Unmarshal(body, &request) != nil {
						t.Fatal("invalid actor request")
					}
					response = codex.RemoteCheckResponse{ID: request.ID, Facts: map[string]string{"owner": "same"}}
					if scenario == "wrong-response-id" {
						response.ID = "foreign"
					}
					if scenario == "lost-response" || scenario == "lost-response-no-record" {
						return nil, errors.New("lost receipt")
					}
					written := len(payload)
					if scenario == "partial-write" {
						written--
					}
					result["send"] = map[string]any{"handle": "actor", "accepted": true, "bytesWritten": written}
				case "read":
					reads++
					if scenario == "permanent-read-failure" || scenario == "read-failure-once" && reads == 1 {
						return nil, errors.New("native_cli_request_unverified")
					}
					if scenario == "lost-response-no-record" {
						response = codex.RemoteCheckResponse{}
					}
					if scenario == "wrong-runtime" {
						runtime = "foreign"
					}
					if (scenario == "identity-read-once" || scenario == "identity-changed-after-retry") && reads > 1 {
						response = codex.RemoteCheckResponse{}
					}
					body, _ := json.Marshal(response)
					line := codex.RemoteCheckPrefix + base64.RawURLEncoding.EncodeToString(body)
					if scenario == "prefixed-frame" {
						line = "echoed input fragment> " + line
					}
					result["terminal"] = map[string]any{"handle": "actor", "tail": []string{line}}
				}
				return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": runtime}, "result": result})
			}})
			budget := 100 * time.Millisecond
			if scenario == "read-failure-once" || scenario == "show-failure-once" || scenario == "identity-read-once" || scenario == "identity-changed-after-retry" {
				budget = time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			facts, err := s.Verify(ctx, "permissions", contracts.CheckResources{})
			if scenario == "normal" || scenario == "lost-response" || scenario == "read-failure-once" || scenario == "show-failure-once" || scenario == "identity-read-once" || scenario == "prefixed-frame" {
				wantShows := 2
				if scenario == "show-failure-once" || scenario == "identity-read-once" {
					wantShows = 3
				}
				if err != nil || facts["owner"] != "same" || shows != wantShows {
					t.Fatal("original response not verified", facts, err, shows)
				}
			} else if err == nil {
				t.Fatal("unverified actor response promoted", scenario)
			}
			if scenario == "identity-read-once" && reads != 1 {
				t.Fatal("original response was discarded", reads)
			}
			if sends != 1 {
				t.Fatal("ambiguous native work was resent", sends)
			}
		})
	}
}

func TestRemoteEnvironmentCheckWrongRuntimeBlocksBeforeDispatch(t *testing.T) {
	inputs := 0
	s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}, actorReady: true}
	s.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "send" {
			inputs++
		}
		return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "foreign"}, "result": map[string]any{"terminal": map[string]string{"handle": "actor", "incarnationId": "incarnation"}}})
	}})
	_, err := s.Execute(context.Background(), "head_push", contracts.CheckResources{})
	if err == nil || inputs != 0 {
		t.Fatal("foreign remote runtime received input", err, inputs)
	}
}

func TestRemoteEnvironmentStartRequiresPositiveNoInputEvidence(t *testing.T) {
	for _, mode := range []string{"rejected", "legacy-unknown", "contradictory-turn"} {
		t.Run(mode, func(t *testing.T) {
			var response codex.RemoteCheckResponse
			s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation"}, actorReady: true}
			sends := 0
			s.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				result := map[string]any{}
				switch args[1] {
				case "show":
					result["terminal"] = map[string]string{"handle": "actor", "incarnationId": "incarnation"}
				case "send":
					sends++
					var payload string
					for i, arg := range args {
						if arg == "--text" {
							payload = args[i+1]
						}
					}
					raw, _ := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(payload, "\r"))
					var request codex.RemoteCheckRequest
					if json.Unmarshal(raw, &request) != nil || request.Method != "start" {
						t.Fatal("unexpected request")
					}
					response = codex.RemoteCheckResponse{ID: request.ID, Error: "authentication_expired_or_unavailable", InputNotSent: mode != "legacy-unknown"}
					if mode == "contradictory-turn" {
						response.Turn = "original-turn"
					}
					result["send"] = map[string]any{"handle": "actor", "accepted": true, "bytesWritten": len(payload)}
				case "read":
					raw, _ := json.Marshal(response)
					result["terminal"] = map[string]any{"handle": "actor", "tail": []string{codex.RemoteCheckPrefix + base64.RawURLEncoding.EncodeToString(raw)}}
				default:
					t.Fatal("unexpected actor call", args)
				}
				return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "original"}, "result": result})
			}})
			_, err := s.Start(context.Background())
			var rejected *codex.EnvironmentInputRejectedError
			if err == nil || errors.As(err, &rejected) != (mode == "rejected") || sends != 1 {
				t.Fatal("no-input evidence classification changed", mode, err, sends)
			}
		})
	}
}

func TestRemoteEnvironmentCleanupKeepsOriginalResourcesOnLostResponse(t *testing.T) {
	s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}, actorReady: true}
	s.client = native.New(native.Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
		return nil, errors.New("connection lost")
	}})
	original := contracts.CheckResources{BaseRef: "refs/heads/steerline-check/original/base", BaseSHA: strings.Repeat("b", 40), BaseOwned: true, HeadRef: "refs/heads/steerline-check/original/head", HeadSHA: strings.Repeat("c", 40), HeadOwned: true, PRNumber: 123, PRURL: "https://github.com/owner/repo/pull/123"}
	result, err := s.Cleanup(context.Background(), "cleanup_head", original)
	if err == nil || result != original {
		t.Fatal("lost remote cleanup discarded original ownership", result, err)
	}
}

func TestRemoteEnvironmentStartupFailureReturnsWithoutDispatchAndClosesOriginalActor(t *testing.T) {
	inputs, closes := 0, 0
	s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}}
	s.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
		result := map[string]any{}
		switch args[1] {
		case "show":
			result["terminal"] = map[string]string{"handle": "actor", "incarnationId": "incarnation"}
		case "read":
			result["terminal"] = map[string]any{"handle": "actor", "tail": []string{codex.RemoteCheckStartupFailurePrefix + "remote_check_configuration_invalid"}}
		case "send":
			inputs++
		case "close":
			closes++
			result["close"] = map[string]any{"handle": "actor", "ptyKilled": true}
		}
		return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "original"}, "result": result})
	}})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.Prepare(ctx); err == nil || err.Error() != "remote_check_configuration_invalid" || inputs != 0 {
		t.Fatal("startup failure became a timeout or dispatched native work", err, inputs)
	}
	if !s.Stop(ctx) || !s.LocalRemoved() || closes != 1 || inputs != 0 {
		t.Fatal("original failed actor not settled", closes, inputs)
	}
}

func TestRemoteCheckFixedProbeHasBoundedControlDeadline(t *testing.T) {
	for _, method := range []string{"execute", "verify"} {
		t.Run(method, func(t *testing.T) {
			s := &LocalTerminalEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "runtime", Terminal: "actor", Incarnation: "inc"}, actorReady: true}
			var response codex.RemoteCheckResponse
			started := time.Now()
			s.client = native.New(native.Config{Output: func(ctx context.Context, _ time.Duration, args ...string) ([]byte, error) {
				deadline, ok := ctx.Deadline()
				if !ok || deadline.After(started.Add(41*time.Second)) {
					t.Fatal("fixed control inherited model execution budget", deadline)
				}
				result := map[string]any{}
				switch args[1] {
				case "show":
					result["terminal"] = map[string]string{"handle": "actor", "incarnationId": "inc"}
				case "send":
					var payload string
					for i, arg := range args {
						if arg == "--text" {
							payload = args[i+1]
						}
					}
					raw, _ := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(payload, "\r"))
					var request codex.RemoteCheckRequest
					if json.Unmarshal(raw, &request) != nil || request.Method != method {
						t.Fatal("unexpected control")
					}
					response = codex.RemoteCheckResponse{ID: request.ID, Facts: map[string]string{"owner": "same"}}
					result["send"] = map[string]any{"handle": "actor", "accepted": true, "bytesWritten": len(payload)}
				case "read":
					raw, _ := json.Marshal(response)
					result["terminal"] = map[string]any{"handle": "actor", "tail": []string{codex.RemoteCheckPrefix + base64.RawURLEncoding.EncodeToString(raw)}}
				default:
					t.Fatal("unexpected operation", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": "runtime"}})
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			r, err := s.call(ctx, codex.RemoteCheckRequest{Method: method, Action: "permissions"})
			if err != nil || r.Facts["owner"] != "same" {
				t.Fatal(r, err)
			}
		})
	}
}
