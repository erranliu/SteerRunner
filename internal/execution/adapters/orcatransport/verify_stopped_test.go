package orcatransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestCodexStoppedRequiresNativeExitProofBeyondPTY(t *testing.T) {
	for _, name := range []string{"live-root", "missing-root", "no-readiness", "remote-live-root"} {
		t.Run(name, func(t *testing.T) {
			environment := ""
			if name == "remote-live-root" {
				environment = "paired-host"
			}
			l, _, s := fixtureOn(t, environment)
			receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, receipt)
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			current, err := process.CurrentIdentity()
			if err != nil {
				t.Fatal(err)
			}
			if name != "no-readiness" {
				state.RemoteReadiness = &codex.RemoteReadiness{SessionID: "original-session", Process: *current}
			}
			if name == "missing-root" {
				state.RemoteReadiness.Process = b.ProcessIdentity{}
			}
			s.Implementation, err = l.envelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s)
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if len(args) != 5 || args[0] != "terminal" || args[1] != "list" {
					t.Fatalf("verification caused an effect: %v", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
			}})
			err = l.verifyStopped(context.Background(), s)
			if err == nil || !strings.Contains(err.Error(), "PTY exit is insufficient") {
				t.Fatal("native exit was inferred from PTY absence", err)
			}
			if process.Probe(current).State != "matches" {
				t.Fatal("live root fixture exited")
			}
			facts, err := l.ShutdownLifecycle(context.Background(), s, nil)
			if err == nil || facts.Stopped {
				t.Fatal("shutdown overstated native resource settlement", facts, err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("read-only probe changed snapshot")
			}
		})
	}
}
