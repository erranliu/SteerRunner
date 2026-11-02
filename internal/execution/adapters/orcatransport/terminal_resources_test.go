package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestWorkspaceTerminalInventoryRetainsFieldScope(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		for _, field := range []string{"totalCount", "hostScope"} {
			t.Run(environment+"/"+field, func(t *testing.T) {
				l, _, _ := fixtureOn(t, environment)
				calls := 0
				l.client = native.New(native.Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
					calls++
					return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []native.Terminal{{Handle: "original", IncarnationID: "incarnation", WorktreeID: "worktree"}}, "truncated": false, field: "invalid unconsumed extension"}, "_meta": map[string]string{"runtimeId": "runtime"}})
				}})
				terminal, err := l.soleWorkspaceTerminal(context.Background(), "worktree", "runtime")
				if err != nil || terminal.Handle != "original" || calls != 1 {
					t.Fatal("sole-terminal reader added field preconditions", terminal, calls, err)
				}
			})
		}
	}
}

func TestTerminalResourceErrorMapping(t *testing.T) {
	p := b.ProcessIdentity{RuntimeID: "runtime", Terminal: "original", Incarnation: "incarnation"}
	client := native.New(native.Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
		return []byte(`{"ok":true,"result":{"terminal":{"handle":"replacement","incarnationId":"incarnation"},"close":{"handle":"replacement","ptyKilled":true}},"_meta":{"runtimeId":"runtime"}}`), nil
	}})
	_, inspectErr := InspectBoundTerminal(context.Background(), client, p)
	_, closeErr := CloseTerminalChecked(context.Background(), client, p.Terminal, p.RuntimeID)
	for _, err := range []error{inspectErr, closeErr} {
		var mapped *b.BackendError
		if !errors.As(err, &mapped) || mapped.Code != b.BackendIdentityMismatch {
			t.Fatal("original execution identity mapping lost", err)
		}
	}
}
