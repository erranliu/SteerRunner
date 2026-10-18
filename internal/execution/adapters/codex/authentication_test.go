package codexworker

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestAuthenticationMissingBlocksBeforeNativeStartAndBusinessInput(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "worker")
	admin := filepath.Join(root, "admin")
	for _, path := range []string{work, admin} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+admin), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := newAuthentication(nil, "https://github.com/erranliu/SteerlineTest.git")
	if err != nil {
		t.Fatal(err)
	}
	a.resolveHost = func(context.Context) ([]byte, error) { return nil, fmt.Errorf("authentication_host_login_unavailable") }
	calls := 0
	backend := Backend{Config: Config{Executable: os.Args[0], Workspace: work, RunDir: root, RunID: strings.Repeat("a", 32), ServerDir: filepath.Join(root, "server"), Sandbox: "workspace-write", WindowsSandbox: "mxc", WorkerRuntime: true, Authentication: a}, Process: ProcessPort{Start: func(*exec.Cmd) (b.StopHandle, error) { calls++; return nil, nil }, WriteAtomic: func(string, []byte) error { calls++; return nil }}}
	_, binding, err := backend.PrepareSession(context.Background(), Binding{}, b.TurnInput{CommandID: strings.Repeat("b", 32), Prompt: "business input", LinkedWorktreeArgs: []string{"--add-dir", root, "--add-dir", root, "--add-dir", root, "--add-dir", admin}})
	if !isPreparationBlocked(err) || !strings.Contains(err.Error(), "authentication_host_login_unavailable") || calls != 0 || binding.Process != nil || binding.SessionID != "" {
		t.Fatalf("authentication failure reached native input: %+v %v calls=%d", binding, err, calls)
	}
	if _, err = os.Stat(filepath.Join(root, "runs")); !os.IsNotExist(err) {
		t.Fatal("business input materialized after authentication failed")
	}
	if a.closed {
		t.Fatal("missing credential permanently consumed preparation; a fresh explicit resume must retain the frozen reference")
	}
}

func TestAuthenticationExpiredReconnectRejectsNewInputOnOriginalSession(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "worker")
	admin := filepath.Join(root, "admin")
	for _, dir := range []string{work, admin} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+admin), 0600); err != nil {
		t.Fatal(err)
	}
	a := testAuthentication(t)
	a.expires = time.Now().Add(-time.Minute)
	called := false
	l := &ExecutionLifecycle{Backend: Backend{Config: Config{Executable: os.Args[0], Workspace: work, RunDir: root, RunID: strings.Repeat("a", 32), ServerDir: filepath.Join(root, "server"), Sandbox: "workspace-write", WindowsSandbox: "mxc", WorkerRuntime: true, Authentication: a}, Process: ProcessPort{Start: func(*exec.Cmd) (b.StopHandle, error) { called = true; return nil, nil }}}}
	process := b.ProcessIdentity{Kind: "os", PID: 123, Created: "original"}
	binding := Binding{Process: &process, Endpoint: "ws://127.0.0.1:1", Generation: "original-generation", SessionID: "00000000-0000-0000-0000-000000000008"}
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{External: process}, Session: b.SessionContext{ProjectID: strings.Repeat("a", 32), ExecutionID: strings.Repeat("b", 32), AttemptID: strings.Repeat("c", 32)}, ConfigurationDigest: "fixed-config", ResourceGeneration: strings.Repeat("d", 32)}
	s.Implementation = l.bindingReceipt(s, binding).Implementation
	input := b.TurnInput{CommandID: strings.Repeat("e", 32), Prompt: "new stage input", LinkedWorktreeArgs: []string{"--add-dir", root, "--add-dir", root, "--add-dir", root, "--add-dir", admin}}
	receipt, observer, err := l.startTurn(context.Background(), s, input)
	if !isPreparationBlocked(err) || receipt.Disposition != b.OperationRejected || observer != nil || called || receipt.State.SessionID != binding.SessionID || receipt.State.InputAccepted || receipt.State.TurnStarted {
		t.Fatalf("expiry changed identity or attempted new input: %+v %v", receipt, err)
	}
}

func TestAuthenticationIssuanceScopeExpiryAndCleanup(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyText := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	now := time.Now().UTC().Truncate(time.Second)
	for _, name := range []string{"valid", "missing-key", "invalid-key", "unauthorized", "expired", "wrong-repository", "multiple-repositories", "extra-permission", "malformed", "long-expiry"} {
		t.Run(name, func(t *testing.T) {
			issued, revoked := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					if r.Header.Get("Authorization") != "Bearer sandbox-test-secret" {
						t.Error("wrong revoke credential")
					}
					revoked++
					w.WriteHeader(204)
					return
				}
				issued++
				if r.URL.Path != "/app/installations/7/access_tokens" || r.Method != "POST" {
					t.Error("wrong issuance target")
				}
				parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
				if len(parts) != 3 {
					t.Error("missing JWT")
					return
				}
				digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
				signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
				if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature) != nil {
					t.Error("invalid App signature")
				}
				var request struct {
					Repositories []string          `json:"repositories"`
					Permissions  map[string]string `json:"permissions"`
				}
				if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Repositories) != 1 || request.Repositories[0] != "SteerlineTest" || len(request.Permissions) != 2 || request.Permissions["contents"] != "write" || request.Permissions["pull_requests"] != "write" {
					t.Error("issuance exceeded fixed scope")
				}
				if name == "unauthorized" {
					w.WriteHeader(401)
					fmt.Fprint(w, "private-key-that-must-not-be-logged")
					return
				}
				w.WriteHeader(201)
				if name == "malformed" {
					fmt.Fprint(w, "{bad private-key-that-must-not-be-logged")
					return
				}
				expires := now.Add(time.Hour)
				repos := []map[string]string{{"full_name": "erranliu/SteerlineTest"}}
				permissions := map[string]string{"contents": "write", "pull_requests": "write", "metadata": "read"}
				switch name {
				case "expired":
					expires = now.Add(-time.Minute)
				case "long-expiry":
					expires = now.Add(2 * time.Hour)
				case "wrong-repository":
					repos[0]["full_name"] = "other/private"
				case "multiple-repositories":
					repos = append(repos, map[string]string{"full_name": "other/private"})
				case "extra-permission":
					permissions["administration"] = "write"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"token": "sandbox-test-secret", "expires_at": expires, "repositories": repos, "permissions": permissions})
			}))
			defer server.Close()
			a, err := newAuthentication(&AuthenticationConfig{AppID: 3, InstallationID: 7, PrivateKeyEnv: "TEST_APP_KEY"}, "https://github.com/erranliu/SteerlineTest.git")
			if err != nil {
				t.Fatal(err)
			}
			a.endpoint = server.URL
			a.now = func() time.Time { return now }
			a.lookup = func(string) string {
				if name == "missing-key" {
					return ""
				}
				if name == "invalid-key" {
					return "not a PEM"
				}
				return keyText
			}
			err = a.prepare(context.Background())
			if (err == nil) != (name == "valid") {
				t.Fatalf("unexpected issuance: %v", err)
			}
			if err != nil && (strings.Contains(err.Error(), "sandbox-test-secret") || strings.Contains(err.Error(), "private-key-that")) {
				t.Fatal("secret in diagnostic")
			}
			if name == "valid" {
				if err = a.prepare(context.Background()); err != nil || issued != 1 {
					t.Fatal("reissued active lease")
				}
				a.now = func() time.Time { return now.Add(59 * time.Minute) }
				if _, err = a.environment(nil); err == nil {
					t.Fatal("expired lease remained usable")
				}
			}
			a.Close()
			a.Close()
			if a.token != "" {
				t.Fatal("secret retained after close")
			}
			if name == "valid" && revoked != 1 {
				t.Fatal("lease not revoked exactly once")
			}
			if (name == "expired" || name == "wrong-repository" || name == "multiple-repositories" || name == "extra-permission" || name == "long-expiry") && revoked != 1 {
				t.Fatal("invalid grant not revoked")
			}
		})
	}
}

func testAuthentication(t *testing.T) *AuthenticationRuntime {
	t.Helper()
	a, err := newAuthentication(&AuthenticationConfig{AppID: 1, InstallationID: 2, PrivateKeyEnv: "TEST_APP_KEY"}, "https://github.com/erranliu/SteerlineTest.git")
	if err != nil {
		t.Fatal(err)
	}
	a.token = "sandbox-test-secret"
	a.expires = time.Now().Add(time.Hour)
	return a
}

func TestAuthenticationEnvironmentAndArgumentsNeverExportSecrets(t *testing.T) {
	a := testAuthentication(t)
	r := &workerRuntime{Shell: os.Args[0], Temp: t.TempDir(), Auth: a}
	env, err := r.environment([]string{"PATH=tools", "TEST_APP_KEY=private-key-that-must-not-escape", "GH_TOKEN=caller-token", "GITHUB_TOKEN=caller-token", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraheader", "GIT_CONFIG_VALUE_0=caller-token"})
	if err != nil {
		t.Fatal(err)
	}
	if environmentValue(env, "TEST_APP_KEY") != "" || environmentValue(env, "GITHUB_TOKEN") != "" || strings.Contains(strings.Join(env, "\n"), "caller-token") {
		t.Fatal("inherited credentials escaped")
	}
	if environmentValue(env, "GH_TOKEN") != a.token || environmentValue(env, "GH_HOST") != "github.com" || environmentValue(env, "GIT_CONFIG_VALUE_3") != "" || environmentValue(env, "GIT_CONFIG_KEY_4") != "http.https://github.com/erranliu/SteerlineTest.git.extraheader" {
		t.Fatal("scoped Git authentication missing")
	}
	args, err := r.configArgs(env)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + a.token))
	for _, secret := range []string{a.token, encoded, "private-key-that-must-not-escape", "caller-token"} {
		if strings.Contains(strings.Join(args, " "), secret) {
			t.Fatal("credential entered argv")
		}
	}
	if strings.Contains(strings.Join(args, " "), "shell_environment_policy.set.GH_TOKEN=") || strings.Contains(strings.Join(args, " "), "shell_environment_policy.set.GIT_CONFIG_VALUE_4=") {
		t.Fatal("secret was pinned through native config")
	}
}

func TestSandboxReadinessRequiresAuthenticatedNativeClients(t *testing.T) {
	a := testAuthentication(t)
	script, _, err := a.readinessScript([]string{t.TempDir()}, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, "Invoke-WebRequest") || strings.Contains(script, "api.github.com/meta") {
		t.Fatal("unrelated anonymous HTTP quota still gates authenticated tool readiness")
	}
	for _, required := range []string{"$env:GH_TOKEN", "gh api installation/repositories", "gh api 'repos/", "git ls-remote --exit-code"} {
		if !strings.Contains(script, required) {
			t.Fatal("authenticated native qualification removed", required)
		}
	}
}

func TestSandboxReadinessUsesNativePolicyAndFiniteEvidence(t *testing.T) {
	for _, stage := range []string{"ready", "tool_environment", "git_admin_write", "network", "github_identity", "remote_git", "untrusted-secret"} {
		t.Run(stage, func(t *testing.T) {
			a := testAuthentication(t)
			dir := t.TempDir()
			backend := Backend{Config: Config{Authentication: a, Workspace: dir, Sandbox: "workspace-write"}, Process: ProcessPort{WriteAtomic: func(path string, data []byte) error { return os.WriteFile(path, data, 0600) }}}
			h := &SessionHandle{Paths: TurnPaths{TurnDir: dir}, Binding: Binding{Process: &b.ProcessIdentity{Kind: "os", PID: 7, Created: "original"}}}
			input := b.TurnInput{CommandID: "original", LinkedWorktreeArgs: []string{"--add-dir", dir, "--add-dir", dir, "--add-dir", dir, "--add-dir", dir}}
			calls := 0
			err := backend.checkSandboxReadiness(context.Background(), h, input, func(ctx context.Context, method string, params any, result any) error {
				calls++
				if method != "command/exec" {
					t.Fatal("probe created a business turn")
				}
				m := params.(map[string]any)
				if m["env"] != nil || m["cwd"] != dir {
					t.Fatal("probe changed tool environment/workspace")
				}
				policy := m["sandboxPolicy"].(map[string]any)
				if policy["type"] != "workspaceWrite" || policy["networkAccess"] != true || len(policy["writableRoots"].([]string)) != 4 {
					t.Fatal("probe widened policy")
				}
				body, _ := json.Marshal(params)
				if strings.Contains(string(body), a.token) {
					t.Fatal("secret in probe argv")
				}
				exit := 1
				if stage == "ready" {
					exit = 0
				}
				raw, _ := json.Marshal(map[string]any{"exitCode": exit, "stdout": fmt.Sprintf(`{"verified":%t,"stage":%q}`, stage == "ready", stage), "stderr": "sandbox-test-secret"})
				return json.Unmarshal(raw, result)
			})
			if calls != 1 || (err == nil) != (stage == "ready") {
				t.Fatalf("unexpected probe: %v", err)
			}
			body, readErr := os.ReadFile(filepath.Join(dir, "sandbox-readiness.json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if strings.Contains(string(body), a.token) || strings.Contains(string(body), "untrusted-secret") {
				t.Fatal("native output entered evidence")
			}
		})
	}
}
