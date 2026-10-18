package codexworker

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteAppBrokerRetainsGrantAndRevokesExactlyOnce(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	for _, mode := range []string{"valid", "wrong-source-repository", "revoke-rejected"} {
		t.Run(mode, func(t *testing.T) {
			var issued, revoked atomic.Int32
			issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					revoked.Add(1)
					if r.Header.Get("Authorization") != "Bearer remote-installation-grant" {
						t.Error("revocation did not use original grant")
					}
					if mode == "revoke-rejected" {
						w.WriteHeader(503)
					} else {
						w.WriteHeader(204)
					}
					return
				}
				issued.Add(1)
				w.WriteHeader(201)
				_ = json.NewEncoder(w).Encode(map[string]any{"token": "remote-installation-grant", "expires_at": time.Now().UTC().Add(30 * time.Minute), "repositories": []map[string]string{{"full_name": "owner/repo"}}, "permissions": map[string]string{"contents": "write", "pull_requests": "write", "metadata": "read"}})
			}))
			defer issuer.Close()
			a, err := newAuthentication(&AuthenticationConfig{AppID: 3, InstallationID: 7, PrivateKeyEnv: "EXECUTION_HOST_APP_KEY"}, "https://github.com/owner/repo.git")
			if err != nil {
				t.Fatal(err)
			}
			a.repositoryBroker = true
			a.endpoint = issuer.URL
			a.lookup = func(name string) string {
				if name != "EXECUTION_HOST_APP_KEY" {
					t.Error("wrong key reference")
				}
				return pemKey
			}
			a.brokerTransport = brokerTransport(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer remote-installation-grant" {
					t.Error("upstream did not use installation grant")
				}
				name := "owner/repo"
				if mode == "wrong-source-repository" {
					name = "foreign/repo"
				}
				body := fmt.Sprintf(`{"full_name":%q,"node_id":"owned-repository","fork":false}`, name)
				if r.URL.Path == "/installation/repositories" {
					body = `{"total_count":1,"repositories":[{"full_name":"owner/repo"}]}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			err = a.prepare(context.Background())
			if mode == "wrong-source-repository" {
				if err == nil || revoked.Load() != 1 {
					t.Fatal("foreign grant broker admitted", err, revoked.Load())
				}
				a.Close()
				return
			}
			if err != nil || a.token == "remote-installation-grant" || a.broker == nil || a.installationToken != "remote-installation-grant" {
				t.Fatal("installation credential escaped its broker", err)
			}
			env, err := a.environment([]string{"EXECUTION_HOST_APP_KEY=" + pemKey, "GH_TOKEN=inherited-secret", "GITHUB_TOKEN=inherited-secret"})
			if err != nil || strings.Contains(strings.Join(env, "\n"), "remote-installation-grant") || strings.Contains(strings.Join(env, "\n"), pemKey) || strings.Contains(strings.Join(env, "\n"), "inherited-secret") {
				t.Fatal("worker inherited upstream credentials", err)
			}
			raw, _ := json.Marshal(a.broker.facts())
			if strings.Contains(string(raw), "remote-installation-grant") || strings.Contains(string(raw), pemKey) || a.broker.facts()["mode"] != "github-app-repository-broker" {
				t.Fatal("secret or wrong source in finite broker facts")
			}
			request := httptest.NewRequest(http.MethodGet, "https://api.github.com/installation/repositories", nil)
			request.Header.Set("Authorization", "Bearer "+a.token)
			response := a.broker.forward(request)
			if response.StatusCode != 200 {
				t.Fatal("installation scope readiness denied", response.StatusCode)
			}
			response.Body.Close()
			if a.broker.permit(httptest.NewRequest(http.MethodGet, "https://api.github.com/user", nil)) || a.broker.permit(httptest.NewRequest(http.MethodGet, "https://api.github.com/repos/foreign/repo", nil)) {
				t.Fatal("App broker admitted host or foreign identity")
			}
			closed := a.Close()
			if closed != (mode != "revoke-rejected") || a.Close() != closed || issued.Load() != 1 || revoked.Load() != 1 || a.token != "" || a.installationToken != "" {
				t.Fatal("App revocation was repeated, omitted or falsely confirmed", closed, issued.Load(), revoked.Load())
			}
		})
	}
}

func TestRemoteAuthenticationStopUsesOnlyOriginalProtectedOwner(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LOCALAPPDATA", state)
	work := filepath.Join(state, "work")
	dir := RemoteRuntimeDirectory(work)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := process.CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	sha, err := RuntimeBinarySHA256()
	if err != nil {
		t.Fatal(err)
	}
	cfg := RemoteAgentConfig{RepositoryURL: "https://github.com/owner/repo.git", Executable: "codex.exe"}
	ready := RemoteReadiness{Verified: true, NativeTransport: RemoteNativeTransport, Contract: PolicyContract, Repository: cfg.RepositoryURL, AuthenticationControl: true, Workspace: work, SessionID: "01a1453f-0327-7f21-b0b9-ee79dad66108", ConfigurationDigest: RemoteConfigurationDigest(cfg), RuntimeSHA256: sha, Process: *identity}
	write := func(name string, v any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("configuration.json", cfg)
	write("ready.json", ready)
	a := testAuthentication(t)
	var revoked atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { revoked.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	a.endpoint = server.URL
	a.onClose = func(ok bool) {
		write("authentication-cleanup.json", map[string]any{"session_id": ready.SessionID, "configuration_digest": ready.ConfigurationDigest, "revocation_confirmed": ok, "broker": map[string]any{"closed": ok, "socket_cleanup_confirmed": ok, "mode": "github-app-repository-broker"}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go watchRemoteAuthentication(ctx, dir, ready, a)
	if _, err := RevokeRemoteAuthentication(ctx, RemoteAuthenticationQuery{Workspace: work, Session: "foreign"}); err == nil {
		t.Fatal("foreign session stopped original authentication")
	}
	if revoked.Load() != 0 {
		t.Fatal("foreign request reached credential owner")
	}
	proof, err := RevokeRemoteAuthentication(ctx, RemoteAuthenticationQuery{Workspace: work, Session: ready.SessionID})
	if err != nil || !proof.Revoked || revoked.Load() != 1 {
		t.Fatal("original revocation missing", err, revoked.Load())
	}
	if _, err = RevokeRemoteAuthentication(ctx, RemoteAuthenticationQuery{Workspace: work, Session: ready.SessionID}); err != nil || revoked.Load() != 1 {
		t.Fatal("revocation replay repeated its effect", err, revoked.Load())
	}
}
