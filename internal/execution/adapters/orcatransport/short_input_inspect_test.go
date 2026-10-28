package orcatransport

import (
	"context"
	"encoding/json"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Independent read-only diagnostic of the sole attempted probe. No lifecycle
// Execute, model input, terminal send or acceptance threshold adjustment.
func TestShortInputProbeInspect(t *testing.T) {
	root := os.Getenv("STEERRUNNER_SHORT_INPUT_INSPECT")
	if root == "" {
		t.Skip("read-only opt-in diagnostic")
	}
	raw, err := os.ReadFile(filepath.Join(root, "target.json"))
	if err != nil {
		t.Fatal(err)
	}
	var target struct{ Runtime, Environment, CLI string }
	if err = json.Unmarshal(raw, &target); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(root, "actual-cli-003.json"))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Stdout struct {
			Result struct{ Terminal native.Terminal }
		}
	}
	if err = json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	terminal := created.Stdout.Result.Terminal
	identity := b.ProcessIdentity{Kind: "orca_terminal", Executable: target.CLI, Terminal: terminal.Handle, RuntimeID: target.Runtime, Incarnation: terminal.IncarnationID, WorktreeID: terminal.WorktreeID, ExecutionHost: (Profile{Environment: target.Environment}).Target().ExecutionHost}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	state, err := native.QueryState(ctx, target.Environment, target.Runtime, "id:"+terminal.WorktreeID)
	record := map[string]any{"query_at": time.Now().UTC(), "state": state, "state_error": errorText(err)}
	if err == nil {
		decoded, e := native.DecodeOriginalState(state, identity)
		record["decoded"], record["decode_error"] = decoded, errorText(e)
		if e == nil {
			sample, e := native.QueryOriginalSample(ctx, target.Environment, identity, decoded.PaneKey)
			record["original_sample"], record["sample_error"] = sample, errorText(e)
			if e == nil {
				// Diagnostic boundary only: the original second input already passed its
				// frozen native baseline. Never promote this wall clock into a receipt.
				raw, _ = os.ReadFile(filepath.Join(root, "actual-cli-006.json"))
				var gate struct{ Started time.Time }
				_ = json.Unmarshal(raw, &gate)
				input, e := native.QueryNativeInput(ctx, target.Environment, target.Runtime, sample.ProviderSession.ID, sample.ProviderSession.TranscriptPath, gate.Started.UnixMilli())
				record["input"], record["input_error"] = input, errorText(e)
				if input != nil {
					record["public_pages"] = input.Pages
				}
			}
		}
	}
	// Cleanup may already have closed the original terminal. The recorded
	// provider identity still permits a read of its public transcript; this
	// supplies diagnostic evidence, never a reconstructed lifecycle receipt.
	if record["public_pages"] == nil {
		raw, _ = os.ReadFile(filepath.Join(root, "native-result.json"))
		var saved struct{ Ready native.AgentState }
		_ = json.Unmarshal(raw, &saved)
		raw, _ = os.ReadFile(filepath.Join(root, "actual-cli-006.json"))
		var gate struct{ Started time.Time }
		_ = json.Unmarshal(raw, &gate)
		input, e := native.QueryNativeInput(ctx, target.Environment, target.Runtime, saved.Ready.ProviderSession.ID, saved.Ready.ProviderSession.TranscriptPath, gate.Started.UnixMilli())
		record["input"], record["input_error"], record["source"] = input, errorText(e), "recorded_original_provider_after_cleanup; diagnostic_only"
		if input != nil {
			record["public_pages"] = input.Pages
		}
	}
	raw, _ = json.MarshalIndent(record, "", "  ")
	if err = os.WriteFile(filepath.Join(root, "independent-native-inspection.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
