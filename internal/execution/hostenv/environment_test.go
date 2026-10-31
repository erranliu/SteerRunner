package hostenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostNativeAuthChildEnvironment(t *testing.T) {
	if os.Getenv("SR_HOSTENV_CHILD") == "1" {
		if os.Getenv("GH_TOKEN") != "host-test-credential" || os.Getenv("HOME") != "host-test-home" || os.Getenv("GIT_CONFIG_VALUE_0") != "" || os.Getenv("GIT_CONFIG_VALUE_1") != "host-test-header" || os.Getenv("GCM_INTERACTIVE") != "Never" || os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("GIT_DIR") != "" || os.Getenv("ORCA_PAIRING_CODE") != "" {
			os.Exit(3)
		}
		os.Exit(0)
	}
	base := append(os.Environ(), "GH_TOKEN=host-test-credential", "HOME=host-test-home", "GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=credential.helper", "GIT_CONFIG_VALUE_0=", "GIT_CONFIG_KEY_1=http.example.extraheader", "GIT_CONFIG_VALUE_1=host-test-header", "OPENAI_API_KEY=provider-test-secret", "GIT_DIR=foreign", "ORCA_PAIRING_CODE=pairing-test-secret")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHostNativeAuthChildEnvironment$")
	cmd.Env = append(Tools(base), "SR_HOSTENV_CHILD=1")
	if err := cmd.Run(); err != nil {
		t.Fatal("host authentication or tool isolation did not survive child launch", err)
	}
}

func TestHostNativeAuthGitInheritedConfiguration(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(git, "config", "--get", "credential.helper")
	cmd.Dir = t.TempDir()
	cmd.Env = Tools(append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=credential.helper", "GIT_CONFIG_VALUE_0=host-native-test-helper"))
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "host-native-test-helper" {
		t.Fatal("Git did not receive the existing host helper")
	}
}

func TestHostNativeAuthDedupAndScopes(t *testing.T) {
	original := []string{"gh_token=old", "GH_TOKEN=new", "GIT_WORK_TREE=foreign", "HTTPS_PROXY=https://proxy.example", "OPENAI_API_KEY=model-key"}
	env := Development(original)
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, "GH_TOKEN=") {
			count++
			if entry != "GH_TOKEN=new" {
				t.Fatal("wrong host credential selected")
			}
		}
	}
	if count != 1 || original[0] != "gh_token=old" {
		t.Fatal("duplicate environment or input modified")
	}
	if !strings.Contains(strings.Join(env, "\n"), "OPENAI_API_KEY=model-key") {
		t.Fatal("Agent model configuration lost")
	}
	if strings.Contains(strings.Join(Tools(env), "\n"), "model-key") {
		t.Fatal("provider credential entered tools")
	}
}

func TestHostNativeAuthMissingLoginFailsWithoutPrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root := t.TempDir()
	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	cmd.Env = Tools(append(os.Environ(), "HOME="+root, "USERPROFILE="+root, "GIT_CONFIG_GLOBAL="+filepath.Join(root, "empty-config"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=credential.helper", "GIT_CONFIG_VALUE_0="))
	cmd.Stdin = strings.NewReader("protocol=https\nhost=host-native-auth.invalid\n\n")
	out, err := cmd.Output()
	if ctx.Err() != nil {
		t.Fatal("missing login waited for interactive authentication")
	}
	if err == nil || len(out) != 0 {
		t.Fatal("missing login unexpectedly returned credentials")
	}
}
