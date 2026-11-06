package orcatransport

import (
	"encoding/json"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestHostNativeStateCodecRejectsRetiredAndAmbiguousFields(t *testing.T) {
	for _, version := range []int{5, 6} {
		if _, err := decodeState(json.RawMessage(`{"turns":[]}`), version); err == nil {
			t.Fatal("retired codec admitted", version)
		}
	}
	for _, version := range []int{StateVersion, LocalStateVersion} {
		for _, raw := range []string{`{"turns":[{"native":{"diagnostic":"old"}}]}`, `{"turns":[{"native":{"conflict":"changed","waiting":"pending"}}]}`} {
			if _, err := decodeState(json.RawMessage(raw), version); err == nil {
				t.Fatal("ambiguous proof admitted", version)
			}
		}
	}
}

func TestHostNativeStateCodecRoundTripRetainsOriginalBinding(t *testing.T) {
	l, host, s := fixture(t)
	s = frozenWorkspaceFixture(t, l, host, s)
	input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: "Implement the bound Spec and Plan."}
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	state.Turns = []Turn{{Command: input.CommandID, Prompt: input.Prompt, InputDigest: digest([]byte(input.Prompt)), Attempted: true, Accepted: true, Native: NativeProof{Agent: "codex", Pane: "tab:pane", Provider: "provider"}}}
	state.Turns[0].Native.Conflict = "Native original completion/input provenance is unverified"
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := l.state(s)
	if err != nil || restored.Process != state.Process || restored.Turns[0].Prompt != input.Prompt {
		t.Fatal("current state lost original identity", err)
	}
	// The current reader preserves the frozen rejection; there is no legacy
	// execution/query calculator to authorize a valid-looking later end.
	if restored.Turns[0].Native != state.Turns[0].Native {
		t.Fatal("roundtrip changed the saved original rejection")
	}
	current, err := l.envelope(s, restored)
	if err != nil || current[0].CodecVersion != LocalStateVersion {
		t.Fatal("upgrade did not write current codec", err)
	}
	s.Implementation = current
	if _, err := l.state(s); err != nil {
		t.Fatal("current envelope cannot be reloaded", err)
	}
	// The separately used usage reader shares the same version boundary.
	s.Ref.RunID = s.Session.ExecutionID // The older lifecycle fixture uses different IDs.
	if _, _, usageState, err := UsageBinding(l.setup.ProfileData, s); err != nil || usageState.Turns[0].Native.Conflict == "" {
		t.Fatal("usage reader diverged from execution decoder", err)
	}
}

func TestHostNativeStateCodecRetainsWaitingWithoutRecomputing(t *testing.T) {
	for _, version := range []int{StateVersion, LocalStateVersion, NativeStateVersion} {
		original := State{Turns: []Turn{{Command: "original-command", Native: NativeProof{Agent: "codex", Pane: "original-pane", Provider: "original-provider", Waiting: "Original evidence missing", BaseRootAt: 17, Outcome: "unreported"}}}}
		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeState(raw, version)
		if err != nil || len(decoded.Turns) != 1 || decoded.Turns[0].Native != original.Turns[0].Native {
			t.Fatal("saved waiting or unknown outcome changed", version, err)
		}
		after, err := json.Marshal(decoded)
		if err != nil || string(after) != string(raw) {
			t.Fatal("legacy wire bytes changed", version, err)
		}
	}
}
