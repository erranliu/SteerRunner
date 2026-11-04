package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

func TestStructuredUsageBindsExactOriginalInputAndNativeSource(t *testing.T) {
	for _, scenario := range []string{"local", "remote", "body-changed", "duplicate-input", "unknown-provider-id", "foreign-provider-turn", "resource-changed", "missing-pairing-host", "profile-host-changed"} {
		t.Run(scenario, func(t *testing.T) {
			l, s, f := structuredFixture(t)
			state, err := l.agentState(s)
			if err != nil {
				t.Fatal(err)
			}
			s.Session.ExecutionID = s.Ref.RunID
			l.setup.Snapshot = s
			s.Implementation, err = l.agentEnvelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			s.Ref.WorkspacePath = s.Workspace.Path
			input := *s.PendingTurn
			before := UsageInputDigest(s)
			f.pages[0].Submissions[0].ProviderItemID = "codex:provider-session:native-turn:0"
			receipt, err := l.readStructured(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			s.Implementation = receipt.Implementation
			if UsageInputDigest(s) == before {
				t.Fatal("accepted native identity did not invalidate old usage observation")
			}
			inputs := []b.TurnInput{input}
			var profile AgentSessionProfile
			_ = json.Unmarshal(l.setup.ProfileData[0].Payload, &profile)
			if scenario == "remote" || scenario == "missing-pairing-host" || scenario == "profile-host-changed" {
				profile.Transport.Environment = "id:original-remote"
				profile.PairingHostID = "sha256:" + strings.Repeat("a", 64)
				s.Ref.Target = profile.Transport.Target()
				s.Ref.External.ExecutionHost = s.Ref.Target.ExecutionHost
				if scenario == "missing-pairing-host" {
					profile.PairingHostID = ""
				}
				if scenario == "profile-host-changed" {
					s.Ref.Target.ExecutionHost = "local"
				}
				raw, _ := json.Marshal(profile)
				meta := l.setup.ProfileData[0]
				l.setup.ProfileData[0], err = b.NewImplementationDataEnvelope(meta, raw)
				if err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "body-changed":
				inputs[0].Prompt = strings.ReplaceAll(input.Prompt, "\r\n", "\n")
			case "duplicate-input":
				inputs = append(inputs, input)
			case "unknown-provider-id", "foreign-provider-turn":
				state, err := l.agentState(s)
				if err != nil {
					t.Fatal(err)
				}
				state.ProviderItemID = "unknown:opaque"
				if scenario == "foreign-provider-turn" {
					state.ProviderItemID = "codex:provider-session:another-turn:0"
				}
				s.Implementation, err = l.agentEnvelope(s, state)
				if err != nil {
					t.Fatal(err)
				}
			case "resource-changed":
				s.Ref.External.WorktreeID = "another-worktree"
			}
			binding, transport, original, err := StructuredUsageBinding(l.setup.ProfileData, s, inputs)
			if scenario != "local" && scenario != "remote" {
				if err == nil {
					t.Fatal("unverified usage binding admitted", scenario, binding)
				}
				return
			}
			if err != nil || binding.SessionID != "provider-session" || len(binding.Commands) != 1 || binding.Commands[0].WireSHA256 != parsers.TextHash(input.Prompt) || binding.Commands[0].NativeTurnID != "native-turn" || binding.Commands[0].ID != input.CommandID || original.SessionID != agentSessionID(s) || original.WorktreeID != s.Ref.External.WorktreeID || original.RuntimeID != s.Ref.External.RuntimeID || transport.Environment != profile.Transport.Environment {
				t.Fatal(binding, original, err)
			}
			if scenario == "remote" && original.HostID != profile.PairingHostID || scenario == "local" && original.HostID != "local" {
				t.Fatal("paired and local host identities confused", original)
			}
			// Exercise the unchanged native parser with the new binding, including
			// CRLF, trailing whitespace and the original native turn constraint.
			binding.HostID, binding.Namespace = "fixture-host", "fixture-account"
			rawRecords := []any{
				map[string]any{"type": "session_meta", "payload": map[string]any{"id": binding.SessionID, "cwd": binding.CWD, "model_provider": "openai"}},
				map[string]any{"type": "event_msg", "payload": map[string]any{"type": "task_started", "turn_id": "native-turn"}},
				map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": input.Prompt}}}},
				map[string]any{"type": "turn_context", "payload": map[string]any{"turn_id": "native-turn", "model": "fixture-model"}},
				map[string]any{"type": "token_usage_record", "payload": map[string]any{"session_id": binding.SessionID, "turn_id": "native-turn", "response_id": "response-original", "usage": map[string]int{"input_tokens": 2, "cached_input_tokens": 0, "cache_write_input_tokens": 0, "output_tokens": 3, "reasoning_output_tokens": 0, "total_tokens": 5}}},
			}
			var log strings.Builder
			for _, record := range rawRecords {
				raw, _ := json.Marshal(record)
				log.Write(raw)
				log.WriteByte('\n')
			}
			parsed := parsers.Parse(context.Background(), binding, []byte(log.String()), false)
			if !parsed.Complete || len(parsed.Events) != 1 || parsed.Events[0].CommandID != input.CommandID || parsed.Events[0].TurnID != "native-turn" || parsed.Events[0].ResponseID != "response-original" || *parsed.Events[0].Tokens.Total != 5 {
				t.Fatal("new binding lost native response attribution", parsed)
			}
			binding.Commands[0].NativeTurnID = "foreign-turn"
			if parsed := parsers.Parse(context.Background(), binding, []byte(log.String()), false); parsed.Complete || len(parsed.Events) != 0 {
				t.Fatal("foreign native turn counted", parsed)
			}
			var candidate native.UsageSession
			raw, _ := json.Marshal(map[string]any{"executionHostId": "local", "structuredSession": map[string]string{"sessionId": original.SessionID, "workspaceId": original.WorktreeID}})
			_ = json.Unmarshal(raw, &candidate)
			if !original.Matches(candidate) {
				t.Fatal("public direct link rejected")
			}
			candidate.StructuredSession.WorkspaceID = "foreign"
			if original.Matches(candidate) {
				t.Fatal("foreign workspace accepted")
			}
			candidate.StructuredSession.WorkspaceID = original.WorktreeID
			candidate.ExecutionHostID = "another-host"
			if original.Matches(candidate) {
				t.Fatal("foreign server execution host accepted")
			}
			candidate.StructuredSession = nil
			if original.Matches(candidate) {
				t.Fatal("missing direct source link guessed")
			}
		})
	}
}
