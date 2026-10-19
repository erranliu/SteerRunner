package livetest

import (
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
)

func TestInitializationReadinessFaultPreservesOriginalOwnerAndEnvelope(t *testing.T) {
	raw := []byte(`{"ok":true,"result":{"terminal":{"handle":"original","tail":["SR_REMOTE_OWNER:original-key","SR_REMOTE_READY:original-ready","unchanged"],"incarnationId":"original-incarnation"}},"_meta":{"runtimeId":"original-runtime"}}`)
	delivered, hidden, err := WithholdReadiness(raw)
	if err != nil || !hidden || strings.Contains(string(delivered), codex.RemoteReadyPrefix) || !strings.Contains(string(delivered), codex.RemoteOwnerPrefix) {
		t.Fatal("fault did not isolate readiness", string(delivered), err)
	}
	var response struct {
		OK     bool
		Result struct {
			Terminal struct {
				Handle, IncarnationID string
				Tail                  []string
			}
		}
		Meta struct {
			RuntimeID string `json:"runtimeId"`
		} `json:"_meta"`
	}
	if json.Unmarshal(delivered, &response) != nil || !response.OK || response.Meta.RuntimeID != "original-runtime" || response.Result.Terminal.Handle != "original" || response.Result.Terminal.IncarnationID != "original-incarnation" || len(response.Result.Terminal.Tail) != 2 {
		t.Fatal("fault changed original identities")
	}
	if !strings.Contains(string(raw), codex.RemoteReadyPrefix) {
		t.Fatal("fault destroyed original source evidence")
	}
	for _, invalid := range []string{`{}`, `{"result":{"terminal":{"tail":null}}`, `{"result":{"terminal":{"tail":42}}}`} {
		if _, _, e := WithholdReadiness([]byte(invalid)); e == nil {
			t.Fatal("invalid fault source accepted", invalid)
		}
	}
}
