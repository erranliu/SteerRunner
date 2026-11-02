package orcatransport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestAgentLaunchReceiptUsesCoreIdentity(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, host, s := fixtureOn(t, environment)
			l.setup.AgentLaunchCommand = "opaque-start"
			host.commandMode, host.delayLaunchIdentity = true, true
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			original, sends := l.client, 0
			l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				raw, err := original.Raw(ctx, timeout, args...)
				if err != nil || len(args) < 2 || args[0] != "terminal" || args[1] != "send" {
					return raw, err
				}
				sends++
				var packet map[string]any
				if err := json.Unmarshal(raw, &packet); err != nil {
					return nil, err
				}
				prompt := packet["result"].(map[string]any)["send"].(map[string]any)["prompt"].(map[string]any)
				prompt["stages"], prompt["provider"], prompt["observation"] = 42, []any{}, map[string]any{}
				return json.Marshal(packet)
			}})
			intent := b.ResourceIntent{Purpose: "agent-start", Name: "original-terminal", Objective: l.setup.AgentLaunchCommand}
			r, _, err = l.Execute(context.Background(), request(s, b.LifecycleCreateResource, b.LifecycleInput{Resource: &intent}))
			if err != nil || r.Disposition != b.OperationAccepted || !r.State.ResourcePending || sends != 1 {
				t.Fatal("identity-only launch acquired a business-stage requirement", r, err, sends)
			}
		})
	}
}
