package codexworker

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var githubRepository = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

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

// AuthenticationRuntime owns one Run's broker capability. The execution host's
// gh credential stays in the broker and is never serialized or given to tools.
type AuthenticationRuntime struct {
	brokerTransport http.RoundTripper
	checkGate       *checkWriteGate
	mu              sync.Mutex
	repository      string
	remoteURL       string
	token           string
	expires         time.Time
	now             func() time.Time
	closed          bool
	onClose         func(bool)
	revoked         bool
	broker          *authenticationBroker
	resolveHost     func(context.Context) ([]byte, error)
}

func newAuthentication(remote string) (*AuthenticationRuntime, error) {
	repository, err := githubTarget(remote)
	if err != nil {
		return nil, err
	}
	return &AuthenticationRuntime{repository: repository, remoteURL: remote, now: time.Now}, nil
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
	resolve := a.resolveHost
	if resolve == nil {
		resolve = hostGitHubCredential
	}
	var transport http.RoundTripper = a.brokerTransport
	if transport == nil {
		upstream := http.DefaultTransport.(*http.Transport).Clone()
		upstream.Proxy = nil // never recurse into the worker proxy
		transport = upstream
	}
	broker, err := prepareHostBroker(ctx, a.repository, resolve, transport)
	if err != nil {
		return err
	}
	broker.checkGate = a.checkGate
	a.broker, a.token, a.expires = broker, broker.capability, broker.expires
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
	clean := []string{}
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_") || strings.HasPrefix(upper, "GH_") || strings.HasPrefix(upper, "GITHUB_") || strings.HasSuffix(upper, "_PROXY") || upper == "SSL_CERT_FILE" || upper == "SSL_CERT_DIR" {
			continue
		}
		clean = append(clean, entry)
	}
	return setEnvironmentValues(clean, map[string]string{"GH_TOKEN": a.token, "GH_HOST": "github.com", "GH_PROMPT_DISABLED": "1", "GIT_TERMINAL_PROMPT": "0"}), nil
}
func (a *AuthenticationRuntime) Close() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return a.revoked
	}
	confirmed := a.token == ""
	if a.broker != nil {
		confirmed = a.broker.close()
	}
	a.token = ""
	a.closed, a.revoked = true, confirmed
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
