package codexworker

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"testing"
)

func TestLegacyAppProfilesRejectWithoutHostFallback(t *testing.T) {
	for _, value := range []string{`null`, `{"access_mode":"full_access"}`, `{"app_id":3,"installation_id":7,"private_key_env":"OLD_KEY"}`} {
		raw := []byte(`{"executable":"codex.exe","github_auth":` + value + `}`)
		if _, err := DecodeProfileConfig(raw); err == nil {
			t.Fatal("persisted App profile silently became host login")
		}
		var remote RemoteAgentConfig
		if err := contracts.DecodeStrict(raw, &remote); err == nil {
			t.Fatal("remote App configuration silently became host login")
		}
	}
}
