package codexworker

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// AuthenticationConfig contains references only. Resolution happens on the
// execution host; neither the App key nor an installation token is serialized.
type AuthenticationConfig struct {
	AppID          int64  `json:"app_id"`
	InstallationID int64  `json:"installation_id"`
	PrivateKeyEnv  string `json:"private_key_env"`
}

var environmentReference = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)
var githubRepository = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func (c AuthenticationConfig) Validate() error {
	if c.AppID <= 0 || c.InstallationID <= 0 || !environmentReference.MatchString(c.PrivateKeyEnv) {
		return errors.New("github_auth requires positive App/installation IDs and a private_key_env reference")
	}
	return nil
}

func githubTarget(remote string) (string, error) {
	u, err := url.Parse(remote)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("sandbox authentication requires the fixed HTTPS github.com remote")
	}
	repository := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	if !githubRepository.MatchString(repository) || strings.Contains(repository, "..") {
		return "", errors.New("invalid sandbox authentication repository")
	}
	return repository, nil
}

// AuthenticationRuntime is process-local, never part of an envelope. For host
// login, token is an opaque broker capability, never the host OAuth credential.
type AuthenticationRuntime struct {
	mu          sync.Mutex
	config      *AuthenticationConfig
	repository  string
	remoteURL   string
	token       string
	expires     time.Time
	client      *http.Client
	endpoint    string
	lookup      func(string) string
	now         func() time.Time
	closed      bool
	onClose     func(bool)
	revoked     bool
	broker      *authenticationBroker
	resolveHost func(context.Context) ([]byte, error)
}

func newAuthentication(config *AuthenticationConfig, remote string) (*AuthenticationRuntime, error) {
	repository, err := githubTarget(remote)
	if err != nil {
		return nil, err
	}
	if config != nil {
		if err = config.Validate(); err != nil {
			return nil, err
		}
	}
	return &AuthenticationRuntime{config: config, repository: repository, remoteURL: remote, client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: "https://api.github.com", lookup: os.Getenv, now: time.Now}, nil
}

func (a *AuthenticationRuntime) prepare(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return errors.New("authentication_lease_closed")
	}
	if a.token != "" {
		return a.validLocked()
	}
	if a.config == nil {
		resolve := a.resolveHost
		if resolve == nil {
			resolve = hostGitHubCredential
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil // fixed upstream, never recurse into the worker proxy
		broker, err := prepareHostBroker(ctx, a.repository, resolve, transport)
		if err != nil {
			return err
		}
		a.broker, a.token, a.expires = broker, broker.capability, broker.expires
		return nil
	}
	keyText := []byte(a.lookup(a.config.PrivateKeyEnv))
	defer clear(keyText)
	block, _ := pem.Decode(keyText)
	if block == nil {
		return errors.New("authentication_key_unavailable: execution-host private key reference is missing or invalid")
	}
	defer clear(block.Bytes)
	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = parsed
	} else if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, _ = parsed.(*rsa.PrivateKey)
	}
	if key == nil {
		return errors.New("authentication_key_invalid: expected RSA App key")
	}
	now := a.now()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(5 * time.Minute).Unix(), "iss": a.config.AppID})
	signing := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	hash := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return errors.New("authentication_signing_failed")
	}
	jwt := signing + "." + base64.RawURLEncoding.EncodeToString(signature)
	_, name, _ := strings.Cut(a.repository, "/")
	body, _ := json.Marshal(map[string]any{"repositories": []string{name}, "permissions": map[string]string{"contents": "write", "pull_requests": "write"}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", a.endpoint, a.config.InstallationID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := a.client.Do(req)
	if err != nil {
		return errors.New("authentication_network_failed: App token endpoint unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("authentication_issuance_rejected: GitHub HTTP %d", response.StatusCode)
	}
	var grant struct {
		Token        string            `json:"token"`
		Expires      time.Time         `json:"expires_at"`
		Permissions  map[string]string `json:"permissions"`
		Repositories []struct {
			FullName string `json:"full_name"`
		} `json:"repositories"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&grant) != nil {
		return errors.New("authentication_issuance_invalid")
	}
	// Hold even a malformed grant only long enough to revoke it, never export it.
	a.token, a.expires = grant.Token, grant.Expires
	valid := len(grant.Repositories) == 1 && strings.EqualFold(grant.Repositories[0].FullName, a.repository) && grant.Permissions["contents"] == "write" && grant.Permissions["pull_requests"] == "write"
	for name, value := range grant.Permissions {
		if name != "contents" && name != "pull_requests" && !(name == "metadata" && value == "read") {
			valid = false
		}
	}
	if !valid || a.validLocked() != nil || grant.Expires.After(now.Add(time.Hour+time.Minute)) {
		a.revokeLocked(ctx)
		return errors.New("authentication_scope_or_expiry_invalid: expected one repository and minimal installation permissions")
	}
	return nil
}

func (a *AuthenticationRuntime) validLocked() error {
	if a.closed || a.token == "" || !a.expires.After(a.now().Add(2*time.Minute)) {
		return errors.New("authentication_expired_or_unavailable: no new business input is allowed")
	}
	return nil
}

func (a *AuthenticationRuntime) environment(env []string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.validLocked(); err != nil {
		return nil, err
	}
	// Native tool inheritance is a separate allowlist. Remove all inherited Git
	// configuration/authentication, including caller tokens and the App key.
	clean := []string{}
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_") || strings.HasPrefix(upper, "GH_") || strings.HasPrefix(upper, "GITHUB_") || strings.HasSuffix(upper, "_PROXY") || upper == "SSL_CERT_FILE" || upper == "SSL_CERT_DIR" || a.config != nil && strings.EqualFold(name, a.config.PrivateKeyEnv) {
			continue
		}
		clean = append(clean, entry)
	}
	return setEnvironmentValues(clean, map[string]string{"GH_TOKEN": a.token, "GH_HOST": "github.com", "GH_PROMPT_DISABLED": "1", "GIT_TERMINAL_PROMPT": "0"}), nil
}

func (a *AuthenticationRuntime) revokeLocked(ctx context.Context) bool {
	if a.broker != nil {
		confirmed := a.broker.close()
		a.token = ""
		return confirmed
	}
	confirmed := a.token == ""
	if a.token != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, a.endpoint+"/installation/token", nil)
		req.Header.Set("Authorization", "Bearer "+a.token)
		if response, err := a.client.Do(req); err == nil {
			confirmed = response.StatusCode == http.StatusNoContent
			response.Body.Close()
		}
	}
	a.token = ""
	return confirmed
}

func (a *AuthenticationRuntime) Close() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return a.revoked
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	confirmed := a.revokeLocked(ctx)
	a.closed = true
	a.revoked = confirmed
	if a.onClose != nil {
		a.onClose(confirmed)
	}
	return confirmed
}

type sandboxPreparationError struct{ error }

func preparationBlocked(err error) error { return &sandboxPreparationError{err} }

func isPreparationBlocked(err error) bool {
	var blocker *sandboxPreparationError
	return errors.As(err, &blocker)
}
