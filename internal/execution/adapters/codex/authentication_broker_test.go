package codexworker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type brokerTransport func(*http.Request) (*http.Response, error)

func (f brokerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthenticationBrokerScopeAndSecretBoundary(t *testing.T) {
	const secret = "controlled-host-oauth-secret-never-exported"
	calls := 0
	transport := brokerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if !strings.Contains(r.Header.Get("Authorization"), secret) && r.URL.Host != "github.com" {
			t.Error("upstream did not receive trusted authentication")
		}
		body := `{"ok":true}`
		switch r.URL.Path {
		case "/repos/erranliu/SteerlineTest":
			body = `{"full_name":"erranliu/SteerlineTest","node_id":"target-ID","fork":false}`
		case "/user":
			body = `{"login":"expected-user"}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	broker, err := prepareHostBroker(context.Background(), "erranliu/SteerlineTest", func(context.Context) ([]byte, error) { return []byte(secret), nil }, transport)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.close()
	a, _ := newAuthentication(nil, "https://github.com/erranliu/SteerlineTest.git")
	a.broker, a.token, a.expires = broker, broker.capability, broker.expires
	runtime := &workerRuntime{Shell: os.Args[0], Temp: t.TempDir(), Auth: a}
	if err = broker.installPublicCA(runtime.Temp); err != nil {
		t.Fatal(err)
	}
	env, err := runtime.environment([]string{"PATH=tools", "GH_TOKEN=" + secret, "GITHUB_TOKEN=" + secret, "HTTPS_PROXY=http://untrusted"})
	if err != nil {
		t.Fatal(err)
	}
	args, err := runtime.configArgs(env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(env, "\n"), secret) || strings.Contains(strings.Join(args, " "), secret) || strings.Contains(strings.Join(args, " "), a.token) || environmentValue(env, "HTTPS_PROXY") != "" {
		t.Fatal("secret escaped or model transport overridden")
	}
	body, _ := os.ReadFile(broker.caPath)
	if strings.Contains(string(body), "PRIVATE") || strings.Contains(string(body), secret) {
		t.Fatal("private material persisted")
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(broker.publicCA)
	proxy, _ := url.Parse(broker.address)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy), TLSClientConfig: &tls.Config{RootCAs: pool}}, Timeout: 5 * time.Second}
	cases := []struct {
		method, path, body, cap string
		status                  int
	}{
		{"GET", "https://api.github.com/user", "", a.token, 200},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest", "", a.token, 200},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345", "", a.token, 200},
		{"GET", "https://api.github.com/repos/other/private/pulls/345", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345/files", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345?extra=1", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/0", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/0345", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/-1", "", a.token, 403},
		{"GET", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345/", "", a.token, 403},
		{"POST", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345", "", a.token, 403},
		{"PATCH", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345", "", a.token, 403},
		{"DELETE", "https://api.github.com/repos/erranliu/SteerlineTest/pulls/345", "", a.token, 403},
		{"GET", "https://api.github.com/repos/other/private", "", a.token, 403},
		{"GET", "https://api.github.com/user/installations", "", a.token, 403},
		{"DELETE", "https://api.github.com/repos/erranliu/SteerlineTest", "", a.token, 403},
		{"GET", "https://api.github.com/user", "", "wrong", 401},
		{"GET", "https://github.com/erranliu/SteerlineTest.git/info/refs?service=git-upload-pack", "", a.token, 200},
		{"GET", "https://github.com/other/private.git/info/refs?service=git-upload-pack", "", a.token, 403},
		{"POST", "https://github.com/erranliu/SteerlineTest.git/git-receive-pack", "pack", a.token, 200},
	}
	for _, test := range cases {
		req, _ := http.NewRequest(test.method, test.path, strings.NewReader(test.body))
		req.Header.Set("Authorization", "token "+test.cap)
		if strings.Contains(test.path, "github.com/erranliu/SteerlineTest.git/") {
			req.Header.Set("Authorization", "basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+test.cap)))
		}
		before := calls
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Errorf("%s %s got %d", test.method, test.path, response.StatusCode)
		}
		if test.status != 200 && calls != before {
			t.Error("denied request reached upstream")
		}
	}
	for _, query := range []struct {
		q    string
		vars map[string]any
		ok   bool
	}{
		{`query($o:String!,$n:String!){repository(owner:$o,name:$n){id pullRequests(first:10){nodes{url state}}}}`, map[string]any{"o": "erranliu", "n": "SteerlineTest"}, true},
		{`{ x:repository(owner:"other",name:"private"){id} }`, nil, false},
		{`{ repository(owner:"erranliu",name:"SteerlineTest"){id} hidden:repository(owner:"other",name:"private"){id} }`, nil, false},
		{`{ repository(owner:"erranliu",name:"SteerlineTest"){...leak} } fragment leak on Repository{secret:deployKeys{nodes{id}}}`, nil, false},
		{`query{node(id:"other-ID"){id}}`, nil, false},
		{`query{repository(owner:"erranliu",name:"SteerlineTest"){pullRequest(number:428){url state mergedAt mergeCommit{oid} headRefName headRefOid headRepository{name} headRepositoryOwner{login} baseRefName}}}`, nil, true},
		{`query{repository(owner:"other",name:"private"){pullRequest(number:428){mergeCommit{oid}}}}`, nil, false},
		{`query{repository(owner:"erranliu",name:"SteerlineTest"){pullRequest(number:428){mergeCommit{history(first:10){nodes{oid}}}}}}`, nil, false},
		{`query{repository(owner:"erranliu",name:"SteerlineTest"){pullRequest(number:428){mergeCommit{oid repository{name}}}}}`, nil, false},
		{`query{repository(owner:"erranliu",name:"SteerlineTest"){pullRequest(number:428){mergeCommit{oid(unsafe:true)}}}}`, nil, false},
		{`query($owner:String!,$repo:String!,$branch:String!){repository(owner:$owner,name:$repo){pullRequests(headRefName:$branch,first:30){nodes{url state headRepositoryOwner{id login ...on User{name}} baseRefName headRefName isCrossRepository}} defaultBranchRef{name}}}`, map[string]any{"owner": "erranliu", "repo": "SteerlineTest", "branch": "head"}, true},
		{`query($owner:String!,$name:String!){repository(owner:$owner,name:$name){...metadata parent{...metadata} mergeCommitAllowed squashMergeAllowed rebaseMergeAllowed}} fragment metadata on Repository{id name owner{login} hasIssuesEnabled hasWikiEnabled description viewerPermission defaultBranchRef{name}}`, map[string]any{"owner": "erranliu", "name": "SteerlineTest"}, true},
		{`query($owner:String!,$name:String!){repository(owner:$owner,name:$name){...metadata parent{...metadata}}} fragment metadata on Repository{id description}`, map[string]any{"owner": "other", "name": "private"}, false},
		{`query{viewer{login organizations(first:20){nodes{name}}}}`, nil, false},
		{`mutation($input:CreatePullRequestInput!){createPullRequest(input:$input){pullRequest{id url}}}`, map[string]any{"input": map[string]any{"repositoryId": "target-ID", "baseRefName": "base", "headRefName": "head", "title": "test"}}, true},
		{`mutation($input:CreatePullRequestInput!){createPullRequest(input:$input){pullRequest{id url}}}`, map[string]any{"input": map[string]any{"repositoryId": "target-ID", "baseRefName": "base", "headRefName": "erranliu:codex/head"}}, true},
		{`mutation($input:CreatePullRequestInput!){createPullRequest(input:$input){pullRequest{id url}}}`, map[string]any{"input": map[string]any{"repositoryId": "target-ID", "baseRefName": "base", "headRefName": "other:codex/head"}}, false},
		{`mutation($input:CreatePullRequestInput!){createPullRequest(input:$input){pullRequest{id url}}}`, map[string]any{"input": map[string]any{"repositoryId": "target-ID", "baseRefName": "base", "headRefName": "erranliu:"}}, false},
		{`mutation($input:CreatePullRequestInput!){createPullRequest(input:$input){pullRequest{id url}}}`, map[string]any{"input": map[string]any{"repositoryId": "target-ID", "baseRefName": "base", "headRefName": "erranliu:other:head"}}, false},
		{`mutation($input:CreatePullRequestInput!){createPullRequest(input:$input){pullRequest{id url}}}`, map[string]any{"input": map[string]any{"repositoryId": "other-ID", "baseRefName": "base", "headRefName": "head"}}, false},
		{`mutation{deleteRepository(input:{repositoryId:"target-ID"}){clientMutationId}}`, nil, false},
		{`query{viewer{...cycle}} fragment cycle on User{...cycle}`, nil, false},
	} {
		raw, _ := json.Marshal(map[string]any{"query": query.q, "variables": query.vars})
		if got := broker.permitGraphQL(raw); got != query.ok {
			t.Errorf("GraphQL boundary got %v expected %v: %s", got, query.ok, query.q)
		}
	}
	// Public files, argv and runtime environments never contain host credential.
	filepath.WalkDir(runtime.Temp, func(path string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			body, _ := os.ReadFile(path)
			if strings.Contains(string(body), secret) {
				t.Error("host secret persisted")
			}
		}
		return nil
	})
	broker.mu.Lock()
	broker.expires = time.Now().Add(-time.Second)
	broker.mu.Unlock()
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "token "+a.token)
	if response, err := client.Do(req); err == nil {
		response.Body.Close()
		if response.StatusCode != 403 && response.StatusCode != 401 {
			t.Error("expired capability accepted")
		}
	}
	if !a.Close() || !a.Close() || len(broker.secret) != 0 || broker.capability != "" {
		t.Error("broker cleanup incomplete")
	}
}

func TestAuthenticationBrokerMissingAndWrongTarget(t *testing.T) {
	for _, name := range []string{"missing", "unauthorized", "wrong-target", "fork"} {
		t.Run(name, func(t *testing.T) {
			secret := []byte("controlled-host-secret")
			broker, err := prepareHostBroker(context.Background(), "owner/repo", func(context.Context) ([]byte, error) {
				if name == "missing" {
					return nil, errors.New("authentication_host_login_unavailable")
				}
				return secret, nil
			}, brokerTransport(func(*http.Request) (*http.Response, error) {
				status := 200
				body := `{"full_name":"other/repo","node_id":"ID"}`
				if name == "unauthorized" {
					status = 401
				}
				if name == "fork" {
					body = `{"full_name":"owner/repo","node_id":"ID","fork":true}`
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			if err == nil || broker != nil {
				t.Fatal("invalid host authority accepted")
			}
			if name != "missing" {
				for _, v := range secret {
					if v != 0 {
						t.Fatal("rejected credential retained")
					}
				}
			}
		})
	}
}
func TestExecutionHostGHCreateWireRetainsRepositoryBoundary(t *testing.T) {
	raw, err := os.ReadFile("testdata/gh-v2.101.0-pr-create.json")
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Requests []json.RawMessage `json:"requests"`
	}
	if json.Unmarshal(raw, &wire) != nil || len(wire.Requests) != 3 {
		t.Fatal("missing execution-host metadata, original PR query or create request")
	}
	broker := &authenticationBroker{repository: "erranliu/SteerlineTest", repositoryID: "target-ID"}
	for i, request := range wire.Requests {
		if !broker.permitGraphQL(request) {
			t.Fatalf("execution-host gh request %d rejected", i)
		}
		var changed struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(request, &changed)
		if input, ok := changed.Variables["input"].(map[string]any); ok {
			input["repositoryId"] = "foreign-ID"
		} else {
			changed.Variables["owner"] = "foreign-owner"
		}
		foreign, _ := json.Marshal(changed)
		if broker.permitGraphQL(foreign) {
			t.Fatalf("gh compatibility admitted foreign repository request %d", i)
		}
	}
	for _, query := range []string{
		`{repository(owner:"erranliu",name:"SteerlineTest"){databaseId{secret}}}`,
		`{repository(owner:"erranliu",name:"SteerlineTest"){sshUrl(token:"secret")}}`,
		`{repository(owner:"erranliu",name:"SteerlineTest"){parent{deployKeys{nodes{id}}}}}`,
	} {
		body, _ := json.Marshal(map[string]any{"query": query})
		if broker.permitGraphQL(body) {
			t.Fatal("gh metadata compatibility widened the field boundary")
		}
	}
}
