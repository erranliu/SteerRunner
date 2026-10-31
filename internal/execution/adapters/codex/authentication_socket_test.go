package codexworker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func socketFixture(t *testing.T) (*AuthenticationTransport, *authenticationBroker) {
	t.Helper()
	// Go's per-test directory includes the test name and can exceed AF_UNIX.
	state, err := os.MkdirTemp("", "sr-a-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(state) })
	plan, err := PlanAuthenticationTransport(state, filepath.Join(state, "work"), "https://github.com/owner/repo.git", PolicyResourceIdentity{ProjectID: "project", ExecutionID: "run", AttemptID: "attempt", Generation: "generation", ConfigDigest: "config"})
	if err != nil {
		t.Fatal(err)
	}
	broker, err := prepareHostBroker(context.Background(), "owner/repo", func(context.Context) ([]byte, error) { return []byte("controlled-host-secret-never-exported"), nil }, brokerTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"login":"owner"}`
		if strings.Contains(r.URL.Path, "/repos/") {
			body = `{"full_name":"owner/repo","node_id":"repository-id"}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { broker.close() })
	return plan, broker
}

func TestAuthenticationSocketLayoutAndIdentity(t *testing.T) {
	plan, broker := socketFixture(t)
	// Reproduce the reported state-directory length, with Git paths exceeding
	// 106 bytes. The transport does not depend on either Git or workspace depth.
	state := filepath.Join(plan.StateDirectory, strings.Repeat("x", 58-len(plan.StateDirectory)))
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanAuthenticationTransport(state, filepath.Join(state, "workspaces", strings.Repeat("w", 26)), plan.Remote, plan.Identity)
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(state, "r", strings.Repeat("r", 26), ".git", "objects", ".sr-auth-12345678.sock")
	if len(old) != 124 || len(filepath.Join(plan.Directory, "s")) != 90 {
		t.Fatal("fixture did not cross socket limit", old, plan.Directory)
	}
	public := t.TempDir()
	if err := broker.installAPITransport(public, plan); err != nil {
		t.Fatal(err)
	}
	if err := broker.installAPITransport(public, plan); err != nil {
		t.Fatal("same lease not reusable", err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", broker.socketPath)
	}}, Timeout: 3 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	for _, token := range []string{"foreign-capability", broker.capability} {
		req, _ := http.NewRequest("GET", "http://api.github.com/user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		want := 401
		if token == broker.capability {
			want = 200
		}
		if response.StatusCode != want {
			t.Fatalf("capability boundary: got %d want %d", response.StatusCode, want)
		}
	}
	otherID := plan.Identity
	otherID.AttemptID = "another-attempt"
	other, err := PlanAuthenticationTransport(state, plan.Workspace, plan.Remote, otherID)
	if err != nil {
		t.Fatal(err)
	}
	if other.Directory == plan.Directory {
		t.Fatal("attempt directories aliased")
	}
	if err := broker.installAPITransport(public, other); err == nil {
		t.Fatal("broker adopted foreign identity")
	}
	if !broker.close() || !broker.close() {
		t.Fatal("cleanup not idempotent")
	}
	if _, err := os.Lstat(plan.Directory); !os.IsNotExist(err) {
		t.Fatal("owned socket directory remains", err)
	}
}

func TestAuthenticationSocketRejectsRedirectedDirectories(t *testing.T) {
	plan, _ := socketFixture(t)
	outside := t.TempDir()
	parent := filepath.Dir(plan.Directory)
	var err error
	if runtime.GOOS == "windows" {
		out, callErr := exec.Command("cmd", "/c", "mklink", "/J", parent, outside).CombinedOutput()
		if callErr != nil {
			t.Fatalf("junction fixture: %v %s", callErr, out)
		}
	} else {
		err = os.Symlink(outside, parent)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := acquireAuthenticationSocket(plan); err == nil {
		t.Fatal("redirected authentication parent accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("redirect target changed", entries, err)
	}
}

func TestAuthenticationSocketPreservesForeignScratchOnClose(t *testing.T) {
	plan, broker := socketFixture(t)
	if err := broker.installAPITransport(t.TempDir(), plan); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(plan.Directory, "pr-body.md")
	body := []byte("foreign PR body must not be recursively removed")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if broker.close() || broker.close() {
		t.Fatal("nonempty transport directory falsely reported as cleaned")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(body) {
		t.Fatal("foreign scratch changed during authentication cleanup", err)
	}
	if _, err := os.Lstat(filepath.Join(plan.Directory, "s")); !os.IsNotExist(err) {
		t.Fatal("original socket was not removed", err)
	}
	if !broker.closed || broker.capability != "" || len(broker.secret) != 0 {
		t.Fatal("cleanup uncertainty retained authentication authority")
	}
}

func TestAuthenticationSocketRejectsReplacementDirectory(t *testing.T) {
	plan, broker := socketFixture(t)
	if err := broker.installAPITransport(t.TempDir(), plan); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(plan.Directory, plan.Directory+"-moved"); err != nil {
		// Windows OpenRoot pins the directory against rename/delete. Verify
		// that the attempted substitution did not damage the original lease.
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		if err := broker.socketLease.verify(); err != nil {
			t.Fatal(err)
		}
		if !broker.close() {
			t.Fatal("pinned directory did not clean up")
		}
		return
	}
	if err := os.Mkdir(plan.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(plan.Directory, "s")
	if err := os.WriteFile(foreign, []byte("foreign socket replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := broker.socketLease.verify(); err == nil {
		t.Fatal("replacement identity passed")
	}
	if broker.close() {
		t.Fatal("replacement cleanup falsely confirmed")
	}
	if raw, err := os.ReadFile(foreign); err != nil || string(raw) != "foreign socket replacement" {
		t.Fatal("replacement removed", err)
	}
}

func TestAuthenticationSocketPreservesReplacementSocket(t *testing.T) {
	plan, broker := socketFixture(t)
	if err := broker.installAPITransport(t.TempDir(), plan); err != nil {
		t.Fatal(err)
	}
	if err := broker.apiListener.Close(); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(plan.Directory, "s")
	if err := os.Rename(socket, socket+"-moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(socket, []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := broker.socketLease.verify(); err == nil {
		t.Fatal("replacement socket passed")
	}
	if broker.close() {
		t.Fatal("replacement socket cleanup falsely confirmed")
	}
	if raw, err := os.ReadFile(socket); err != nil || string(raw) != "foreign" {
		t.Fatal("replacement socket removed", err)
	}
}

func TestAuthenticationSocketIsolationAndFailedSetup(t *testing.T) {
	plan, broker := socketFixture(t)
	otherID := plan.Identity
	otherID.ExecutionID = "other-run"
	other, err := PlanAuthenticationTransport(plan.StateDirectory, plan.Workspace, plan.Remote, otherID)
	if err != nil {
		t.Fatal(err)
	}
	otherLease, err := acquireAuthenticationSocket(other)
	if err != nil {
		t.Fatal(err)
	}
	defer otherLease.close()
	if err := broker.installAPITransport(t.TempDir(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireAuthenticationSocket(plan); err == nil {
		t.Fatal("duplicate lease was adopted")
	}
	if !broker.close() {
		t.Fatal("owned cleanup failed")
	}
	if err := otherLease.verify(); err != nil {
		t.Fatal("cleanup touched other run", err)
	}

	plan2, broker2 := socketFixture(t)
	badTemp := filepath.Join(plan2.StateDirectory, "file")
	if err := os.WriteFile(badTemp, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := broker2.installAPITransport(badTemp, plan2); err == nil {
		t.Fatal("invalid config directory accepted")
	}
	if _, err := os.Lstat(plan2.Directory); !os.IsNotExist(err) {
		t.Fatal("partial setup leaked socket", err)
	}
	if !broker2.close() {
		t.Fatal("partial setup cleanup failed")
	}
}

func TestAuthenticationSocketPreservesForeignFiles(t *testing.T) {
	plan, broker := socketFixture(t)
	if err := broker.installAPITransport(t.TempDir(), plan); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(plan.Directory, "foreign")
	if err := os.WriteFile(foreign, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if broker.close() || broker.close() {
		t.Fatal("incomplete cleanup falsely confirmed")
	}
	if raw, err := os.ReadFile(foreign); err != nil || string(raw) != "preserve" {
		t.Fatal("foreign file removed", err)
	}
	if _, err := os.Lstat(filepath.Join(plan.Directory, "s")); !os.IsNotExist(err) {
		t.Fatal("owned socket not removed", err)
	}
}

func TestAuthenticationSocketPolicyBinding(t *testing.T) {
	plan, _ := socketFixture(t)
	permissions, _ := NativePermissions("windows", "local", "codex", "full_access")
	payload := PolicyResourcePayload{NativeVersion: PolicyCodexVersion, Contract: PolicyContract, Settings: b.LaunchCommand{Executable: "codex.exe", Args: permissions.Args(), WorkingDirectory: plan.Workspace}, AuthenticationTransport: plan}
	envelope, err := NewPolicyResourceEnvelope(plan.Identity, payload)
	if err != nil {
		t.Fatal(err)
	}
	auth, _ := newAuthentication(plan.Remote)
	lifecycle := &ExecutionLifecycle{Backend: Backend{Config: Config{RunDir: plan.StateDirectory, Workspace: plan.Workspace, Authentication: auth}}}
	snapshot := b.LifecycleSnapshot{PolicyResources: &envelope, ConfigurationDigest: plan.Identity.ConfigDigest, ResourceGeneration: plan.Identity.Generation}
	snapshot.Ref.WorkspacePath = plan.Workspace
	snapshot.Session.ProjectID, snapshot.Session.ExecutionID, snapshot.Session.AttemptID = plan.Identity.ProjectID, plan.Identity.ExecutionID, plan.Identity.AttemptID
	if err := lifecycle.bindAuthenticationTransport(snapshot); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*AuthenticationTransport){
		func(p *AuthenticationTransport) { p.Directory = p.StateDirectory },
		func(p *AuthenticationTransport) { p.Identity.AttemptID = "foreign" },
		func(p *AuthenticationTransport) { p.Identity.Generation = "foreign" },
		func(p *AuthenticationTransport) { p.Workspace += "-foreign" },
		func(p *AuthenticationTransport) { p.Remote = "https://github.com/other/repo.git" },
	} {
		bad := *plan
		mutate(&bad)
		payload.AuthenticationTransport = &bad
		e, err := NewPolicyResourceEnvelope(plan.Identity, payload)
		if err != nil {
			t.Fatal(err)
		}
		snapshot.PolicyResources = &e
		if err := lifecycle.bindAuthenticationTransport(snapshot); err == nil {
			t.Fatal("foreign transport accepted", bad)
		}
	}
	// A legacy resource can be inspected but cannot silently gain a new grant.
	payload.AuthenticationTransport = nil
	e, _ := NewPolicyResourceEnvelope(plan.Identity, payload)
	e.CodecVersion = 3
	e, _ = b.NewImplementationDataEnvelope(e, e.Payload)
	snapshot.PolicyResources = &e
	if _, err := DecodePolicyResourceEnvelope(e, plan.Identity); err != nil {
		t.Fatal("legacy inspection", err)
	}
	if err := lifecycle.bindAuthenticationTransport(snapshot); err == nil {
		t.Fatal("legacy permissions silently expanded")
	}
}

func TestAuthenticationSocketBuildsFrozenResource(t *testing.T) {
	plan, _ := socketFixture(t)
	raw, _ := EncodeProfileConfig(ProfileConfig{Executable: os.Args[0], AccessMode: "full_access"})
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: ProfileCodecKey, CodecVersion: ProfileCodecVersion, Purpose: b.ImplementationConfigPurpose, ProjectID: plan.Identity.ProjectID, ProfileID: "profile", AcceptedConfigVersion: "accepted"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	request := PolicyResourceBuildRequest{ProfileData: []b.ImplementationDataEnvelope{profile}, ProfileIdentity: ProfileConfigIdentity{ProjectID: plan.Identity.ProjectID, ProfileID: "profile", AcceptedConfigVersion: "accepted"}, ResourceIdentity: plan.Identity, Host: "local", Agent: "codex", GOOS: "windows", Workspace: plan.Workspace, RunDirectory: plan.StateDirectory, RepositoryURL: plan.Remote, LinkedWriteArgs: []string{"--add-dir", "objects", "--add-dir", "refs", "--add-dir", "logs", "--add-dir", "admin"}}
	bound, err := BuildPolicyResourceEnvelope(context.Background(), request, func(context.Context, string) (string, error) { return PolicyCodexVersion, nil })
	if err != nil {
		t.Fatal(err)
	}
	payload, err := DecodePolicyResourceEnvelope(bound.Envelope, plan.Identity)
	if err != nil || payload.AuthenticationTransport == nil || *payload.AuthenticationTransport != *plan {
		t.Fatal("resource did not freeze transport", payload, err)
	}
	if slices.Contains(bound.WriteRoots, plan.Directory) {
		t.Fatal("resource grant missing", bound.WriteRoots)
	}
	if _, err := os.Lstat(plan.Directory); !os.IsNotExist(err) {
		t.Fatal("planning created transport effects", err)
	}
	request.RunDirectory = filepath.Join(plan.StateDirectory, strings.Repeat("x", 108))
	if _, err := BuildPolicyResourceEnvelope(context.Background(), request, func(context.Context, string) (string, error) { return PolicyCodexVersion, nil }); err == nil || err.Error() != "authentication_api_socket_path_too_long" {
		t.Fatal("oversized root admitted", err)
	}
}

func TestAuthenticationSocketNativeGrants(t *testing.T) {
	plan, broker := socketFixture(t)
	if err := broker.installAPITransport(t.TempDir(), plan); err != nil {
		t.Fatal(err)
	}
	auth := testAuthentication(t)
	auth.broker = broker
	auth.token = broker.capability
	auth.expires = broker.expires
	runtime := &workerRuntime{Shell: os.Args[0], Temp: t.TempDir(), Auth: auth}
	if err := broker.installPublicCA(runtime.Temp); err != nil {
		t.Fatal(err)
	}
	env, err := runtime.environment([]string{"PATH=tools"})
	if err != nil {
		t.Fatal(err)
	}
	args, err := runtime.configArgs(env)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(args, "--add-dir")
	if i >= 0 {
		t.Fatal("native launch omitted exact authentication grant", args)
	}
	backend := Backend{Config: Config{AuthenticationTransport: plan}}
	gitArgs, _ := LinkedWorktreeWriteArgs("objects", "refs", "logs", "admin")
	roots, _ := WritableRoots(backend.toolWriteArgs(gitArgs))
	if !slices.Equal(roots, []string{"objects", "refs", "logs", "admin", plan.Directory}) {
		t.Fatal("wrong tool grants", roots)
	}
	if slices.Contains(roots, plan.StateDirectory) || slices.Contains(roots, filepath.Dir(plan.Directory)) {
		t.Fatal("shared parent granted")
	}
	// Also check the actual native turn policy compiler, used by commands and turns.
	params, err := TurnStartParams(plan.Workspace, "thread", "command", "prompt", "danger-full-access", "", "", roots)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(params["sandboxPolicy"])
	if strings.Contains(string(raw), "writableRoots") || params["sandboxPolicy"].(map[string]any)["type"] != "dangerFullAccess" {
		t.Fatal(string(raw))
	}
}
