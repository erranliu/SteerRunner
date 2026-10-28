package orcatransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exactly two business inputs through the production lifecycle. Every unknown
// stops here; neither the helper nor cleanup ever sends a supplemental Enter.
func runShortInputSequence(t *testing.T, ctx context.Context, l *Lifecycle, s b.LifecycleSnapshot, creator native.Client, identity b.ProcessIdentity, ready native.AgentState, record map[string]any, root string) {
	trace := newShortInputTrace(t, root)
	trace.attach(l)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	record["probe_executable_sha256"] = digest(binary)
	if deadline, ok := ctx.Deadline(); ok {
		record["probe_deadline"] = deadline
	}
	trace.write("probe.ready", map[string]any{"process": identity, "ready": traceAgent(ready, identity), "snapshot_session": s.Session})
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(nonce)
	first, second, tool := "SR_FIRST_"+id, "SR_PUBLISH_"+id, "SR_TOOL_"+id
	prompts := []string{"Run only the harmless shell command Write-Output '" + tool + "' once, then reply exactly " + first + ". No file writes, business tests, commits, deployment or publication.", "Publication-style delivery probe.\nDo not create a PR, run tools or change anything. Reply only with " + second + "."}
	variant := os.Getenv("STEERRUNNER_SHORT_SEQUENCE_VARIANT")
	if variant == "" {
		variant = "multiline"
	}
	if variant != "multiline" && variant != "single-line" && variant != "reflow" {
		t.Fatal("unknown short sequence variant")
	}
	if variant == "single-line" {
		prompts[1] = strings.ReplaceAll(prompts[1], "\n", " ")
	}
	if variant == "reflow" {
		// Harmless text with a long final display row. Unlike draft833.txt,
		// this live input has no project references or optional file writer.
		prompts[0] += "\nIsolated display diagnostic; the following line is inert context, not additional work.\n" + strings.TrimSuffix(strings.Repeat("Preserve every original space and newline; do not change files or run extra tools. ", 8), " ")
	}
	record["short_variant"] = variant
	expected := []string{first, second}
	attempts := []map[string]any{}
	record["short_attempts"] = &attempts
	for i, prompt := range prompts {
		commandBytes := make([]byte, 16)
		_, err := rand.Read(commandBytes)
		if err != nil {
			t.Fatal(err)
		}
		_, budget, err := m.FreezeInput(m.InputPart{Source: "caller.direct", Owner: m.CallerInput, Text: prompt})
		if err != nil {
			t.Fatal(err)
		}
		input := b.TurnInput{CommandID: hex.EncodeToString(commandBytes), Prompt: prompt, Budget: budget, InputVersion: uint64(i + 1)}
		at := map[string]any{"index": i + 1, "input": input, "prepare_at": time.Now().UTC()}
		attempts = append(attempts, at)
		raw, _ := json.MarshalIndent(input, "", "  ")
		if err = os.WriteFile(filepath.Join(root, "short-input-"+string(rune('1'+i))+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		s.PendingTurn = &input
		req := request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input})
		req.Operation.ID += "-" + identity.Terminal + "-" + input.CommandID
		receipt, _, err := l.Execute(ctx, req)
		trace.write("input.prepared", map[string]any{"input_index": i + 1, "receipt": receipt, "error": errorText(err)})
		at["prepare_receipt"], at["prepare_error"] = receipt, errorText(err)
		if err != nil {
			t.Fatal("short input preparation refused", err)
		}
		bind(&s, receipt)
		state, err := l.state(s)
		if err != nil {
			t.Fatal(err)
		}
		turn := turnFor(&state, input.CommandID)
		at["wire_budget"], at["wire_text"] = turn.WireBudget, nativeWireText(turn)
		at["submit_at"] = time.Now().UTC()
		req = request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})
		req.Operation.ID += "-" + identity.Terminal + "-" + input.CommandID
		receipt, obs, err := l.Execute(ctx, req)
		trace.write("input.submitted", map[string]any{"input_index": i + 1, "receipt": receipt, "error": errorText(err)})
		at["submit_returned_at"], at["submit_receipt"], at["submit_error"] = time.Now().UTC(), receipt, errorText(err)
		record["submission_attempts"] = i + 1
		if receipt.State.InputAccepted {
			record["business_inputs"] = i + 1
		}
		bind(&s, receipt)
		if err != nil || obs == nil {
			trace.failure(l, s, creator, identity, ready.PaneKey, i+1)
			read, readErr := creator.MetaCall(ctx, "terminal", "read", "--terminal", identity.Terminal, "--limit", "80", "--json")
			at["draft_read"], at["draft_read_error"] = read, errorText(readErr)
			t.Fatal("original short submission unknown/refused; no resend", err)
		}
		defer func() {
			obs.Close(context.Background())
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := obs.(*observer).awaitCleanup(cleanup); err != nil {
				t.Error("observer cleanup", err)
			}
		}()
		completed := false
		for !completed {
			select {
			case event, ok := <-obs.Events():
				if !ok {
					select {
					case doneErr := <-obs.Done():
						trace.write("observer.closed", map[string]any{"error": errorText(doneErr)})
					default:
					}
					trace.failure(l, s, creator, identity, ready.PaneKey, i+1)
					t.Fatal("short observer ended before completion")
				}
				if len(event.Implementation) > 0 {
					s.Implementation = event.Implementation
				}
				trace.event(i+1, event, s, l)
				completed = event.Fact.Kind == b.EventCompleted && event.Fact.Completed
				event.Persisted <- nil
				event.Processed <- nil
			case <-ctx.Done():
				obs.Close(context.Background())
				trace.write("observer.timeout", map[string]any{"input_index": i + 1, "error": errorText(ctx.Err())})
				trace.failure(l, s, creator, identity, ready.PaneKey, i+1)
				t.Fatal("short input completion unverified", ctx.Err())
			}
		}
		obs.Close(context.Background())
		if err = obs.(*observer).awaitCleanup(ctx); err != nil {
			t.Fatal(err)
		}
		receipt, _, err = l.ObserveLifecycle(ctx, s)
		trace.write("input.final", map[string]any{"input_index": i + 1, "receipt": receipt, "error": errorText(err)})
		at["completed_at"], at["final_receipt"] = time.Now().UTC(), receipt
		if err != nil || receipt.TurnEvidence == nil || !receipt.TurnEvidence.FinalVerified || strings.TrimSpace(receipt.TurnEvidence.LastMessage) != expected[i] {
			t.Fatal("original short final unverified", err, receipt.Diagnostic)
		}
		bind(&s, receipt)
		sample, err := native.QueryOriginalSample(ctx, l.profile.Environment, identity, ready.PaneKey)
		if err != nil {
			t.Fatal(err)
		}
		verified, err := native.QueryOriginalInput(ctx, l.profile.Environment, identity, ready.PaneKey, turn.Native.BaseRootAt, sample.AgentState)
		at["original_query"], at["query_error"] = verified, errorText(err)
		state, stateErr := l.state(s)
		completedTurn := turnFor(&state, input.CommandID)
		trace.write("input.original_check", map[string]any{"input_index": i + 1, "query": traceAgent(verified, identity), "error": errorText(err), "state_error": errorText(stateErr), "completed_turn": completedTurn})
		if verified.NativeInput != nil {
			at["public_pages"] = verified.NativeInput.Pages
		}
		if err != nil || stateErr != nil || !shortOriginalInputVerified(verified, identity, completedTurn) {
			trace.failure(l, s, creator, identity, ready.PaneKey, i+1)
			t.Fatal("public input differs from frozen original", err)
		}
		if i == 0 {
			pages, _ := json.Marshal(verified.NativeInput.Pages)
			if !strings.Contains(string(pages), `"tool-call"`) || !strings.Contains(string(pages), `"tool-result"`) || !strings.Contains(string(pages), tool) {
				t.Fatal("first harmless tool lacks public evidence")
			}
		}
		// The adapter itself gates the next dispatch on the original composer.
	}
	record["completed"] = true
}

// Production supports both an independently fenced completed-input query and
// the original observed working-root/authority proof. A post-final probe must
// not demand the query-only marker from the latter (which has no such marker).
func shortOriginalInputVerified(s native.AgentState, p b.ProcessIdentity, turn *Turn) bool {
	if turn == nil || s.NativeInput == nil || s.NativeInput.Text != nativeWireText(turn) || !turn.Native.recordedInputMatches(s, turn) {
		return false
	}
	if s.NativeInput.MatchesOriginalQuery(s) {
		return s.NativeInput.MatchesOriginalProcess(p)
	}
	return s.NativeInput.Observation != nil
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
