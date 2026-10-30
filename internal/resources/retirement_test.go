package resources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestPublicResourcesRetireCheckWithoutHidingRunChannels(t *testing.T) {
	catalog := Catalog()
	for _, entry := range catalog.Combinations {
		if entry.Workflow.ID == "" || entry.Workflow.Version < 1 {
			t.Fatal("capability retained an empty workflow", entry)
		}
		execution := contracts.Execution{Adapter: entry.Adapter, Target: contracts.Target{Kind: entry.TargetKind}, Settings: marshal(map[string]any{"agent": entry.Agent, "access_mode": entry.AccessMode})}
		if catalog.Declares(execution, nil, entry.Operations[0]) {
			t.Fatal("capability accepted a missing Run workflow", entry)
		}
	}
	raw, err := json.Marshal(Capabilities())
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"environment-check", "environment_check", "environment_session", "environment_task", "environment_tool", "steerrunner-state/v3", "steerrunner-state/v4"} {
		if strings.Contains(string(raw), retired) {
			t.Fatal("retired capability", retired)
		}
	}
	for _, command := range []string{"submit", "inspect", "observe", "progress-file", "result-file", "stop"} {
		if !strings.Contains(string(raw), `"`+command+`"`) {
			t.Fatal("Run consumer missing", command)
		}
	}
	stopped, err := json.Marshal(contracts.StoppedVerification{})
	if err != nil || strings.Contains(string(stopped), `"checks"`) {
		t.Fatal("empty retired field retained", string(stopped), err)
	}
}
