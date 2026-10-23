package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaStateCodecPreservesLegacyRejections(t *testing.T) {
	for _, diagnostic := range []string{"", "Native main agent is waiting; no next input was sent", "Native original completion/input provenance is unverified", "unrecognized historical diagnostic"} {
		t.Run(diagnostic, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"turns": []any{map[string]any{"native": map[string]any{"pane": "tab:pane", "diagnostic": diagnostic}}}})
			state, err := decodeState(raw, previousStateVersion)
			if err != nil {
				t.Fatal(err)
			}
			p := state.Turns[0].Native
			waiting := diagnostic == "Native main agent is waiting; no next input was sent"
			if p.diagnostic() != diagnostic || (p.Conflict == "") != (waiting || diagnostic == "") {
				t.Fatal("historical evidence reinterpreted", p)
			}
			current, _ := json.Marshal(state)
			if strings.Contains(string(current), `"diagnostic"`) {
				t.Fatal("legacy prose policy leaked into current state")
			}
			restored, err := decodeState(current, StateVersion)
			if err != nil || restored.Turns[0].Native != p {
				t.Fatal("current state changed original evidence", err)
			}
		})
	}
	for _, raw := range []string{
		`{"unknown":true,"turns":[]}`,
		`{"turns":[{"native":{"pane":"p","diagnostic":"wait","unknown":true}}]}`,
		`{"turns":[{"native":{"pane":"p","diagnostic":"wait","conflict":"other"}}]}`,
		`{"turns":[{"native":{"pane":"p","diagnostic":123}}]}`,
	} {
		if _, err := decodeState(json.RawMessage(raw), previousStateVersion); err == nil {
			t.Fatal("invalid legacy state accepted", raw)
		}
	}
	if _, err := decodeState(json.RawMessage(`{"turns":[{"native":{"diagnostic":"old"}}]}`), StateVersion); err == nil {
		t.Fatal("current codec accepted legacy policy")
	}
}

func TestOrcaStateCodecUpgradeRetainsOriginalBinding(t *testing.T) {
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
	// Model an ambiguous v6 field exactly as written by the deployed build.
	state.Turns[0].Native.Conflict = "Native original completion/input provenance is unverified"
	raw, _ := json.Marshal(state)
	raw = []byte(strings.Replace(string(raw), `"conflict":`, `"diagnostic":`, 1))
	old := s.Implementation[0]
	old.CodecVersion = previousStateVersion
	s.Implementation[0], err = b.NewImplementationDataEnvelope(old, raw)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := l.state(s)
	if err != nil || restored.Process != state.Process || restored.Turns[0].Prompt != input.Prompt {
		t.Fatal("upgrade lost original identity", err)
	}
	end := fixtureState(100, input.Prompt, "done")
	end.LastAssistantMessage = "valid-looking later reply"
	if restored.Turns[0].Native.query(end, &restored.Turns[0], time.Now()) {
		t.Fatal("upgrade cleared ambiguous historical rejection")
	}
	current, err := l.envelope(s, restored)
	if err != nil || current[0].CodecVersion != StateVersion {
		t.Fatal("upgrade did not write current codec", err)
	}
	s.Implementation = current
	if _, err := l.state(s); err != nil {
		t.Fatal("current envelope cannot be reloaded", err)
	}
	// The separately used usage reader shares the same version boundary.
	s.Implementation[0] = old
	s.Implementation[0], err = b.NewImplementationDataEnvelope(old, raw)
	if err != nil {
		t.Fatal(err)
	}
	s.Ref.RunID = s.Session.ExecutionID // The older lifecycle fixture uses different IDs.
	if _, _, usageState, err := UsageBinding(l.setup.ProfileData, s); err != nil || usageState.Turns[0].Native.Conflict == "" {
		t.Fatal("usage reader diverged from execution decoder", err)
	}
}
