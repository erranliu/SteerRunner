package codexworker

// The broker is an authentication transport, not a publication executor. Git
// packs and GitHub requests originate in the sandbox. Only this process sees
// the host credential. The child receives a random, expiring repo capability.
import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

type authenticationBroker struct {
	checkGate                               *checkWriteGate
	mu                                      sync.Mutex
	secret                                  []byte
	capability                              string
	repository, repositoryID, login         string
	expires                                 time.Time
	certificate                             tls.Certificate
	publicCA                                []byte
	caPath, address                         string
	listener                                net.Listener
	apiListener                             net.Listener
	configDir, socketPath                   string
	socketLease                             *authenticationSocketLease
	cleanupConfirmed                        bool
	connections                             map[net.Conn]bool
	transport                               http.RoundTripper
	timer                                   *time.Timer
	lifetime                                context.Context
	cancel                                  context.CancelFunc
	closed                                  bool
	allowed, denied                         uint64
	gitRead, gitWrite, prQueries, prCreates uint64
}

func hostGitHubCredential(ctx context.Context) ([]byte, error) {
	executable, err := exec.LookPath("gh")
	if err != nil {
		return nil, errors.New("authentication_host_gh_unavailable")
	}
	// Output is captured inside the trusted process, never logged or returned.
	cmd := exec.CommandContext(ctx, executable, "auth", "token", "--hostname", "github.com")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GH_") && !strings.HasPrefix(strings.ToUpper(key), "GITHUB_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Stderr = io.Discard
	output, err := cmd.Output()
	if err != nil {
		clear(output)
		return nil, errors.New("authentication_host_login_unavailable")
	}
	token := bytes.TrimSpace(output)
	if len(token) < 20 || len(token) > 4096 || bytes.ContainsAny(token, "\r\n\t ") {
		clear(output)
		return nil, errors.New("authentication_host_credential_invalid")
	}
	result := bytes.Clone(token)
	clear(output)
	return result, nil
}

func prepareHostBroker(ctx context.Context, repository string, resolve func(context.Context) ([]byte, error), transport http.RoundTripper) (*authenticationBroker, error) {
	secret, err := resolve(ctx)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			clear(secret)
		}
	}()
	b := &authenticationBroker{secret: secret, repository: repository, expires: time.Now().Add(time.Hour), transport: transport, connections: map[net.Conn]bool{}}
	// Bind both immutable repository ID and identity on the execution host. This
	// is source qualification only; readiness still runs in the actual sandbox.
	var repo struct {
		FullName string `json:"full_name"`
		NodeID   string `json:"node_id"`
		Fork     bool   `json:"fork"`
	}
	if err = b.qualify(ctx, "/repos/"+repository, &repo); err != nil {
		return nil, err
	}
	if !strings.EqualFold(repo.FullName, repository) || repo.NodeID == "" || repo.Fork {
		return nil, errors.New("authentication_target_invalid_or_fork_unsupported")
	}
	var user struct {
		Login string `json:"login"`
	}
	if err = b.qualify(ctx, "/user", &user); err != nil {
		return nil, err
	}
	if user.Login == "" {
		return nil, errors.New("authentication_host_identity_invalid")
	}
	b.repositoryID, b.login = repo.NodeID, user.Login
	capBytes := make([]byte, 32)
	if _, err = rand.Read(capBytes); err != nil {
		return nil, errors.New("authentication_capability_generation_failed")
	}
	b.capability = base64.RawURLEncoding.EncodeToString(capBytes)
	clear(capBytes)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "SteerRunner execution-only authentication"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: b.expires, DNSNames: []string{"api.github.com", "github.com"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	b.certificate = tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	b.publicCA = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	b.listener, err = net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("authentication_broker_listener_unavailable")
	}
	b.address = "http://" + b.listener.Addr().String()
	b.lifetime, b.cancel = context.WithCancel(context.Background())
	server := &http.Server{Handler: http.HandlerFunc(b.connect), ReadHeaderTimeout: 5 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	go server.Serve(b.listener)
	b.timer = time.AfterFunc(time.Until(b.expires), func() { b.close() })
	success = true
	return b, nil
}

func (b *authenticationBroker) qualify(ctx context.Context, path string, destination any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com"+path, nil)
	req.Header.Set("Authorization", "Bearer "+string(b.secret))
	req.Header.Set("Accept", "application/vnd.github+json")
	response, err := b.transport.RoundTrip(req)
	if err != nil {
		return errors.New("authentication_host_network_failed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("authentication_host_access_rejected: HTTP " + strconv.Itoa(response.StatusCode))
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(destination) != nil {
		return errors.New("authentication_host_response_invalid")
	}
	return nil
}

func (b *authenticationBroker) installPublicCA(temp string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("authentication_lease_closed")
	}
	b.caPath = filepath.Join(temp, "authentication-public-ca.pem")
	// Certificate only. Neither its signing key nor the host token is written.
	if err := os.WriteFile(b.caPath, b.publicCA, 0600); err != nil {
		return errors.New("authentication_public_ca_unavailable")
	}
	return nil
}

func (b *authenticationBroker) installAPITransport(temp string, plan *AuthenticationTransport) (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := plan.validate(); err != nil {
		return err
	}
	if b.apiListener != nil {
		if b.socketLease == nil || b.socketLease.plan != *plan || b.configDir != filepath.Join(temp, "gh-public") {
			return errors.New("authentication_api_binding_mismatch")
		}
		return b.socketLease.verify()
	}
	if b.closed {
		return errors.New("authentication_api_transport_unavailable")
	}
	lease, err := acquireAuthenticationSocket(plan)
	if err != nil {
		return err
	}
	b.socketLease = lease
	defer func() {
		if err != nil {
			if b.apiListener != nil {
				_ = b.apiListener.Close()
				b.apiListener = nil
			}
			if !lease.close() {
				err = errors.Join(err, errors.New("authentication_cleanup_unverified"))
			}
		}
	}()
	b.socketPath = filepath.Join(plan.Directory, "s")
	listener, err := net.Listen("unix", b.socketPath)
	if err != nil {
		return errors.New("authentication_api_socket_unavailable")
	}
	listener.(*net.UnixListener).SetUnlinkOnClose(false)
	b.apiListener = listener
	lease.socket, err = lease.root.Lstat("s")
	if err != nil {
		return errors.New("authentication_api_socket_unverified")
	}
	if err = lease.verify(); err != nil {
		return err
	}
	b.configDir = filepath.Join(temp, "gh-public")
	if err = os.MkdirAll(b.configDir, 0700); err != nil {
		return errors.New("authentication_api_config_unavailable")
	}
	if err = os.WriteFile(filepath.Join(b.configDir, "config.yml"), []byte("http_unix_socket: "+strconv.Quote(b.socketPath)+"\n"), 0600); err != nil {
		return errors.New("authentication_api_config_unavailable")
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, ErrorLog: log.New(io.Discard, "", 0), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "api.github.com" {
			http.Error(w, "authentication transport target denied", 403)
			return
		}
		r.URL.Scheme = "https"
		r.URL.Host = "api.github.com"
		response := b.forward(r)
		defer response.Body.Close()
		for key, values := range response.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	}), ConnState: func(c net.Conn, s http.ConnState) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if s == http.StateNew {
			if b.closed {
				c.Close()
			} else {
				b.connections[c] = true
			}
		} else if s == http.StateClosed {
			delete(b.connections, c)
		}
	}}
	go server.Serve(listener)
	return nil
}

func (b *authenticationBroker) close() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return b.cleanupConfirmed
	}
	b.closed = true
	if b.timer != nil {
		b.timer.Stop()
	}
	_ = b.listener.Close()
	if b.cancel != nil {
		b.cancel()
	}
	if b.apiListener != nil {
		_ = b.apiListener.Close()
	}
	for connection := range b.connections {
		_ = connection.Close()
	}
	clear(b.secret)
	b.secret = nil
	b.capability = ""
	b.certificate.PrivateKey = nil
	b.cleanupConfirmed = b.socketLease.close()
	return b.cleanupConfirmed
}

func (b *authenticationBroker) connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "CONNECT" || (r.Host != "api.github.com:443" && r.Host != "github.com:443") {
		http.Error(w, "authentication transport target denied", 403)
		return
	}
	b.mu.Lock()
	if b.closed || len(b.connections) >= 16 || !time.Now().Before(b.expires) {
		b.mu.Unlock()
		http.Error(w, "authentication capability unavailable", 403)
		return
	}
	certificate := b.certificate
	raw, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		b.mu.Unlock()
		return
	}
	b.connections[raw] = true
	b.mu.Unlock()
	defer func() { raw.Close(); b.mu.Lock(); delete(b.connections, raw); b.mu.Unlock() }()
	_ = raw.SetDeadline(time.Now().Add(2 * time.Minute))
	if _, err = io.WriteString(raw, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	secure := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	defer secure.Close() // libcurl/OpenSSL requires TLS close_notify after body
	if secure.Handshake() != nil {
		return
	}
	reader := bufio.NewReaderSize(secure, 32<<10)
	for {
		request, err := http.ReadRequest(reader)
		if err != nil {
			return
		}
		request.URL.Scheme = "https"
		request.URL.Host = r.Host
		if request.Host != strings.TrimSuffix(r.Host, ":443") && request.Host != r.Host {
			request.Body.Close()
			return
		}
		response := b.forward(request)
		err = response.Write(secure)
		response.Body.Close()
		request.Body.Close()
		if err != nil || request.Close || response.Close {
			return
		}
	}
}

func brokerDenied(status int, message string) *http.Response {
	return &http.Response{StatusCode: status, ProtoMajor: 1, ProtoMinor: 1, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"message":` + strconv.Quote(message) + `}`)), Close: true}
}

func (b *authenticationBroker) forward(r *http.Request) *http.Response {
	b.mu.Lock()
	// TLS does not grant authority. Every inner request must carry the worker's
	// capability. Basic is used by Git; gh uses token/Bearer.
	auth := r.Header.Get("Authorization")
	provided := ""
	scheme, value, _ := strings.Cut(auth, " ")
	if strings.EqualFold(scheme, "Basic") {
		decoded, _ := base64.StdEncoding.DecodeString(value)
		_, provided, _ = strings.Cut(string(decoded), ":")
		clear(decoded)
	} else {
		_, provided, _ = strings.Cut(auth, " ")
	}
	valid := !b.closed && time.Now().Before(b.expires) && len(provided) == len(b.capability) && subtle.ConstantTimeCompare([]byte(provided), []byte(b.capability)) == 1
	if !valid {
		b.denied++
		b.mu.Unlock()
		return brokerDenied(401, "execution capability unavailable")
	}
	b.mu.Unlock()
	if !b.permit(r) {
		b.mu.Lock()
		b.denied++
		b.mu.Unlock()
		return brokerDenied(403, "execution repository or operation denied")
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return brokerDenied(401, "execution capability expired")
	}
	secret := string(b.secret)
	b.allowed++
	switch r.URL.Path {
	case "/" + b.repository + ".git/git-upload-pack":
		b.gitRead++
	case "/" + b.repository + ".git/git-receive-pack":
		b.gitWrite++
	case "/graphql":
		// permitGraphQL already parsed and bounded this body. Audit only the
		// operation kind, never variables, headers, body, or capability.
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(body, &request)
		doc, _ := parser.ParseQuery(&ast.Source{Input: request.Query})
		if doc.Operations[0].Operation == ast.Mutation {
			b.prCreates++
		} else {
			b.prQueries++
		}
	default:
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/"+b.repository+"/pulls/") {
			b.prQueries++
		}
	}
	b.mu.Unlock()
	// No redirect following, caller-controlled proxy, auth, cookies, or arbitrary
	// headers reach upstream. Only the two fixed GitHub HTTPS origins are used.
	upstream := r.Clone(context.Background())
	upstream.RequestURI = ""
	upstream.Host = ""
	target := *r.URL
	target.Host = strings.TrimSuffix(target.Host, ":443")
	upstream.URL = &target
	upstream.Header = make(http.Header)
	for _, name := range []string{"Accept", "Content-Type", "Git-Protocol", "User-Agent", "X-GitHub-Api-Version"} {
		if value := r.Header.Get(name); value != "" {
			upstream.Header.Set(name, value)
		}
	}
	upstream.Header.Set("Authorization", "Bearer "+secret)
	if target.Host == "github.com" {
		upstream.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+secret)))
	}
	ctx, cancel := context.WithTimeout(b.lifetime, 90*time.Second)
	upstream = upstream.WithContext(ctx)
	response, err := b.transport.RoundTrip(upstream)
	if err != nil {
		cancel()
		return brokerDenied(502, "execution authentication upstream unavailable")
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		response.Body.Close()
		cancel()
		return brokerDenied(403, "execution authentication redirect denied")
	}
	response.Body = &brokerResponseBody{ReadCloser: response.Body, cancel: cancel}
	filtered := make(http.Header)
	for _, name := range []string{"Content-Type", "Content-Length"} {
		if value := response.Header.Get(name); value != "" {
			filtered.Set(name, value)
		}
	}
	response.Header = filtered
	response.Request = nil
	response.Close = true
	return response
}

func (b *authenticationBroker) facts() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	facts := map[string]any{"mode": "host-gh-repository-broker", "repository": b.repository, "repository_id": b.repositoryID, "identity": b.login, "expires_at": b.expires, "closed": b.closed, "allowed_requests": b.allowed, "denied_requests": b.denied, "git_upload_pack": b.gitRead, "git_receive_pack": b.gitWrite, "pr_queries": b.prQueries, "pr_creates": b.prCreates}
	if b.socketLease != nil {
		facts["authentication_transport"] = b.socketLease.plan
		facts["socket_cleanup_confirmed"] = b.socketLease.cleaned
	}
	return facts
}

type brokerResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *brokerResponseBody) Close() error { b.cancel(); return b.ReadCloser.Close() }

func (b *authenticationBroker) permit(r *http.Request) bool {
	if r.URL.User != nil || r.URL.Fragment != "" || r.URL.RawPath != "" || strings.ContainsAny(r.URL.Path, "\\%") || strings.Contains(r.URL.Path, "..") {
		return false
	}
	if b.checkGate != nil && !b.checkGate.permit(r, b.repository) {
		return false
	}
	host := strings.TrimSuffix(r.URL.Host, ":443")
	path := r.URL.Path
	if host == "github.com" {
		prefix := "/" + b.repository + ".git/"
		if !strings.HasPrefix(path, prefix) {
			return false
		}
		suffix := strings.TrimPrefix(path, prefix)
		if r.Method == "GET" && suffix == "info/refs" {
			q, err := url.ParseQuery(r.URL.RawQuery)
			return err == nil && len(q) == 1 && len(q["service"]) == 1 && (q.Get("service") == "git-upload-pack" || q.Get("service") == "git-receive-pack")
		}
		return r.Method == "POST" && r.URL.RawQuery == "" && (suffix == "git-upload-pack" || suffix == "git-receive-pack")
	}
	if host != "api.github.com" {
		return false
	}
	if r.Method == "GET" && r.URL.RawQuery == "" && (path == "/meta" || path == "/user" || path == "/repos/"+b.repository) {
		return true
	}
	if b.checkGate != nil && strings.HasPrefix(path, "/repos/"+b.repository+"/pulls") {
		return true
	}
	if b.checkGate != nil {
		return false
	}
	// The frozen publication provider may expose a single-PR REST query to
	// the worker. Admit that same-repository read, without admitting PR lists,
	// subresources, query parameters or REST mutations.
	if r.Method == http.MethodGet && r.URL.RawQuery == "" {
		if number, ok := strings.CutPrefix(path, "/repos/"+b.repository+"/pulls/"); ok && number != "" && number[0] != '0' {
			digits := true
			for _, c := range number {
				if c < '0' || c > '9' {
					digits = false
					break
				}
			}
			if n, err := strconv.ParseUint(number, 10, 64); digits && err == nil && n > 0 {
				return true
			}
		}
	}
	if r.Method != "POST" || path != "/graphql" || r.URL.RawQuery != "" {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (128<<10)+1))
	r.Body.Close()
	if err != nil || len(body) > 128<<10 {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return b.permitGraphQL(body)
}

func (b *authenticationBroker) permitGraphQL(body []byte) bool {
	var request struct {
		Query         string         `json:"query"`
		Variables     map[string]any `json:"variables"`
		OperationName string         `json:"operationName"`
	}
	if json.Unmarshal(body, &request) != nil {
		return false
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: request.Query})
	if err != nil || len(doc.Operations) != 1 {
		return false
	}
	op := doc.Operations[0]
	if len(op.Directives) != 0 || op.Operation == ast.Subscription {
		return false
	}
	// Expand fragments with cycle/depth protection. Validate semantic field names,
	// never aliases or query substrings; variables/literals resolve identically.
	var visit func(ast.SelectionSet, string, int) bool
	visit = func(set ast.SelectionSet, kind string, depth int) bool {
		if depth > 16 {
			return false
		}
		for _, selection := range set {
			switch s := selection.(type) {
			case *ast.FragmentSpread:
				f := doc.Fragments.ForName(s.Name)
				if f == nil || len(s.Directives) != 0 || len(f.Directives) != 0 || !visit(f.SelectionSet, kind, depth+1) {
					return false
				}
			case *ast.InlineFragment:
				if len(s.Directives) != 0 || !visit(s.SelectionSet, kind, depth+1) {
					return false
				}
			case *ast.Field:
				if len(s.Directives) != 0 {
					return false
				}
				next := ""
				scalar := false
				switch kind {
				case "query":
					switch s.Name {
					case "viewer":
						next = "owner"
					case "repository":
						owner, name := "", ""
						for _, arg := range s.Arguments {
							v, e := arg.Value.Value(request.Variables)
							if e != nil {
								return false
							}
							value, ok := v.(string)
							if !ok {
								return false
							}
							switch arg.Name {
							case "owner":
								owner = value
							case "name":
								name = value
							default:
								return false
							}
						}
						if !strings.EqualFold(owner+"/"+name, b.repository) || len(s.Arguments) != 2 {
							return false
						}
						next = "repo"
					default:
						return false
					}
				case "mutation":
					if s.Name != "createPullRequest" || len(s.Arguments) != 1 || s.Arguments[0].Name != "input" {
						return false
					}
					value, e := s.Arguments[0].Value.Value(request.Variables)
					if e != nil {
						return false
					}
					input, ok := value.(map[string]any)
					if !ok || input["repositoryId"] != b.repositoryID {
						return false
					}
					for name, v := range input {
						switch name {
						case "repositoryId", "title", "body", "baseRefName", "headRefName":
							if _, ok := v.(string); !ok {
								return false
							}
						case "draft", "maintainerCanModify":
							if _, ok := v.(bool); !ok {
								return false
							}
						default:
							return false
						}
					}
					head, hok := input["headRefName"].(string)
					base, bok := input["baseRefName"].(string)
					if !hok || !bok || head == "" || base == "" {
						return false
					}
					if owner, branch, qualified := strings.Cut(head, ":"); qualified {
						targetOwner, _, _ := strings.Cut(b.repository, "/")
						if !strings.EqualFold(owner, targetOwner) || branch == "" || strings.Contains(branch, ":") {
							return false
						}
					}
					next = "created"
				case "repo":
					switch s.Name {
					case "id", "databaseId", "name", "nameWithOwner", "url", "sshUrl", "hasIssuesEnabled", "hasWikiEnabled", "description", "viewerPermission", "isPrivate", "isFork", "mergeCommitAllowed", "squashMergeAllowed", "rebaseMergeAllowed":
						scalar = true
					case "owner":
						next = "owner"
					case "defaultBranchRef":
						next = "ref"
					case "parent":
						next = "repoMetadata" // qualified non-fork: always null
					case "pullRequests":
						next = "prs"
					case "pullRequest":
						next = "pr"
					default:
						return false
					}
				case "repoMetadata":
					switch s.Name {
					case "id", "databaseId", "name", "nameWithOwner", "sshUrl", "viewerPermission", "isPrivate", "hasIssuesEnabled", "hasWikiEnabled", "description":
						scalar = true
					case "owner":
						next = "owner"
					case "defaultBranchRef":
						next = "ref"
					default:
						return false
					}
				case "prs":
					switch s.Name {
					case "nodes":
						next = "pr"
					case "edges":
						next = "edge"
					case "pageInfo":
						next = "page"
					case "totalCount":
						scalar = true
					default:
						return false
					}
				case "edge":
					if s.Name != "node" {
						return false
					}
					next = "pr"
				case "page":
					scalar = s.Name == "hasNextPage" || s.Name == "endCursor" || s.Name == "hasPreviousPage" || s.Name == "startCursor"
				case "created":
					if s.Name != "pullRequest" {
						return false
					}
					next = "pr"
				case "pr":
					switch s.Name {
					case "id", "number", "url", "state", "title", "body", "baseRefName", "headRefName", "headRefOid", "baseRefOid", "isCrossRepository", "isDraft", "maintainerCanModify", "createdAt", "updatedAt", "closedAt", "mergedAt":
						scalar = true
					case "headRepositoryOwner", "author":
						next = "owner"
					case "headRepository", "repository":
						next = "repoMetadata"
					case "mergeCommit":
						next = "commitMetadata"
					default:
						return false
					}
				case "owner":
					scalar = s.Name == "login" || s.Name == "id" || s.Name == "name" || s.Name == "__typename"
				case "ref":
					scalar = s.Name == "name"
				case "commitMetadata":
					scalar = s.Name == "oid"
				}
				if scalar {
					if len(s.SelectionSet) != 0 {
						return false
					}
				} else if next == "" || len(s.SelectionSet) == 0 || !visit(s.SelectionSet, next, depth+1) {
					return false
				}
				// Only repository scope and PR lookup/list pagination accept arguments.
				if kind != "query" && kind != "mutation" && !(kind == "repo" && (s.Name == "pullRequests" || s.Name == "pullRequest")) && len(s.Arguments) != 0 {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	return visit(op.SelectionSet, string(op.Operation), 0)
}
