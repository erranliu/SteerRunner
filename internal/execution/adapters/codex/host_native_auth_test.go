package codexworker

import (
	"os"
	"strings"
	"testing"
)

func TestHostNativeAuthShellPolicyAndHooks(t *testing.T) {
	runtime := &workerRuntime{Shell: os.Args[0], Temp: t.TempDir(), OrcaHooks: true}
	env, err := runtime.environment([]string{"PATH=tools", "GH_TOKEN=host-test-secret", "HTTPS_PROXY=https://user:proxy-test-secret@proxy.example", "NO_PROXY=example.internal", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.example.extraheader", "GIT_CONFIG_VALUE_0=header-test-secret", "ORCA_AGENT_HOOK_TOKEN=hook-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	args, err := runtime.configArgs(env)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(args, " ")
	for _, secret := range []string{"host-test-secret", "proxy-test-secret", "header-test-secret", "hook-test-secret"} {
		if strings.Contains(text, secret) {
			t.Fatal("host secret serialized in native argv")
		}
	}
	for _, name := range []string{"GH_TOKEN", "GIT_CONFIG_VALUE_*", "ORCA_AGENT_HOOK_TOKEN", "ignore_default_excludes=true"} {
		if !strings.Contains(text, name) {
			t.Fatal("required tool environment omitted", name)
		}
	}
	if environmentValue(env, "GIT_CONFIG_VALUE_0") != "header-test-secret" || !strings.Contains(environmentValue(env, "NO_PROXY"), "example.internal") || !strings.Contains(environmentValue(env, "NO_PROXY"), "127.0.0.1") {
		t.Fatal("host Git or proxy configuration changed")
	}
	runtime.OrcaHooks = false
	args, _ = runtime.configArgs(env)
	if strings.Contains(strings.Join(args, " "), "ORCA_AGENT_HOOK_TOKEN") {
		t.Fatal("hook scope broadened")
	}
}
