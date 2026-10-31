package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
	l, _, s := fixture(t)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	s, input := prepared(t, l, s, strings.Repeat("c", 32))
	r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	state.Turns[0].Native.Conflict = "Native original completion/input provenance is unverified"
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := l.state(s)
	if err != nil || restored.Process != state.Process || restored.Turns[0].Prompt != input.Prompt {
		t.Fatal("current state lost original identity", err)
	}
	end := fixtureState(100, input.Prompt, "done")
	end.LastAssistantMessage = "valid-looking later reply"
	if restored.Turns[0].Native.query(end, &restored.Turns[0], time.Now(), false) {
		t.Fatal("upgrade cleared ambiguous historical rejection")
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
