package codexworker

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// A check lease cannot perform an external write unless the center has saved
// and armed one exact operation. Consume before forwarding: a lost response or
// a second tool invocation cannot dispatch the same write again.
type checkWriteGate struct {
	mu                                      sync.Mutex
	action, ref, oldSHA, newSHA, base, head string
	pr                                      int
	armed                                   bool
}

func (g *checkWriteGate) arm(action, ref, oldSHA, newSHA, base, head string, pr int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.action, g.ref, g.oldSHA, g.newSHA, g.base, g.head, g.pr, g.armed = action, ref, oldSHA, newSHA, base, head, pr, true
}
func (g *checkWriteGate) disarm() { g.mu.Lock(); g.armed = false; g.mu.Unlock() }

func requestBody(r *http.Request, limit int64) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	r.Body.Close()
	if err != nil || int64(len(body)) > limit {
		return nil, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return body, true
}

func (g *checkWriteGate) permit(r *http.Request, repository string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	path := r.URL.Path
	if r.Method == "GET" {
		if strings.HasPrefix(path, "/repos/"+repository+"/pulls") {
			prefix := "/repos/" + repository + "/pulls"
			if path == prefix {
				q := r.URL.Query()
				return len(q) == 4 && q.Get("state") == "all" && q.Get("head") == strings.Split(repository, "/")[0]+":"+g.head && q.Get("base") == g.base && q.Get("per_page") == "100"
			}
			_, err := strconv.Atoi(strings.TrimPrefix(path, prefix+"/"))
			return err == nil && r.URL.RawQuery == ""
		}
		return true // repository scope is checked by the ordinary broker
	}
	if r.Method == "POST" && strings.HasSuffix(path, "/git-upload-pack") {
		return true
	}
	if !g.armed {
		return false
	}
	if r.Method == "POST" && path == "/"+repository+".git/git-receive-pack" {
		body, ok := requestBody(r, 8<<20)
		if !ok || len(body) < 4 {
			return false
		}
		n, err := strconv.ParseUint(string(body[:4]), 16, 16)
		if err != nil || n < 4 || int(n)+4 > len(body) {
			return false
		}
		// Exactly one ref update; no second pkt-line command is accepted.
		line := strings.SplitN(string(body[4:n]), "\x00", 2)[0]
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != g.oldSHA || fields[1] != g.newSHA || fields[2] != g.ref || string(body[n:n+4]) != "0000" {
			return false
		}
		g.armed = false
		return true
	}
	if r.Method == "POST" && path == "/repos/"+repository+"/pulls" && g.action == "pr_create" && r.URL.RawQuery == "" {
		body, ok := requestBody(r, 16<<10)
		if !ok {
			return false
		}
		var v struct {
			Title      string `json:"title"`
			Body       string `json:"body"`
			Base       string `json:"base"`
			Head       string `json:"head"`
			Maintainer bool   `json:"maintainer_can_modify"`
		}
		if json.Unmarshal(body, &v) != nil || v.Base != g.base || v.Head != g.head || v.Maintainer {
			return false
		}
		g.armed = false
		return true
	}
	if r.Method == "PATCH" && path == "/repos/"+repository+"/pulls/"+strconv.Itoa(g.pr) && g.action == "pr_close" && r.URL.RawQuery == "" {
		body, ok := requestBody(r, 1024)
		if !ok {
			return false
		}
		var v map[string]any
		if json.Unmarshal(body, &v) != nil || len(v) != 1 || v["state"] != "closed" {
			return false
		}
		g.armed = false
		return true
	}
	return false
}
