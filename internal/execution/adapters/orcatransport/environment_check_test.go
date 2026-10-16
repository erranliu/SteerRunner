package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"testing"
	"time"
)

func TestRemoteActorControlRequiresFullLineAndOriginalResponseIdentity(t *testing.T) {
	for _, scenario := range []string{"normal", "changed-incarnation", "wrong-runtime", "partial-write", "lost-response", "wrong-response-id"} {
		t.Run(scenario, func(t *testing.T) {
			sends, shows := 0, 0
			var response codex.RemoteCheckResponse
			s := &RemoteEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}, actorReady: true}
			s.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				runtime := "original"
				result := map[string]any{}
				switch args[1] {
				case "show":
					shows++
					incarnation := "incarnation"
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
					if scenario == "lost-response" {
						return nil, errors.New("lost receipt")
					}
					written := len(payload)
					if scenario == "partial-write" {
						written--
					}
					result["send"] = map[string]any{"handle": "actor", "accepted": true, "bytesWritten": written}
				case "read":
					if scenario == "wrong-runtime" {
						runtime = "foreign"
					}
					body, _ := json.Marshal(response)
					result["terminal"] = map[string]any{"handle": "actor", "tail": []string{codex.RemoteCheckPrefix + base64.RawURLEncoding.EncodeToString(body)}}
				}
				return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": runtime}, "result": result})
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			facts, err := s.Verify(ctx, "permissions", contracts.CheckResources{})
			if scenario == "normal" {
				if err != nil || facts["owner"] != "same" || shows != 2 {
					t.Fatal("original response not verified", facts, err, shows)
				}
			} else if err == nil {
				t.Fatal("unverified actor response promoted", scenario)
			}
			if sends != 1 {
				t.Fatal("ambiguous native work was resent", sends)
			}
		})
	}
}

func TestRemoteEnvironmentCheckWrongRuntimeBlocksBeforeDispatch(t *testing.T) {
	inputs := 0
	s := &RemoteEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}, actorReady: true}
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

func TestRemoteEnvironmentCleanupKeepsOriginalResourcesOnLostResponse(t *testing.T) {
	s := &RemoteEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}, actorReady: true}
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
	s := &RemoteEnvironmentSession{setup: codex.RemoteCheckSetup{CheckID: strings.Repeat("a", 32)}, original: b.ProcessIdentity{RuntimeID: "original", Terminal: "actor", Incarnation: "incarnation", WorktreeID: "repo::path"}}
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
