package orca

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestProgressWireBytesAndBounds(t *testing.T) {
	for _, tc := range []struct {
		body  string
		count int
		eof   bool
		valid bool
	}{
		{"", 0, true, true}, {base64.StdEncoding.EncodeToString([]byte{0, 255}), 2, true, true},
		{"eA==", 2, true, false}, {"eA==", 1, false, false}, {strings.Repeat("A", 87388), 65537, true, false}, {"?", 1, true, false},
	} {
		raw, _ := json.Marshal(map[string]any{"contentBase64": tc.body, "bytesRead": tc.count, "eof": tc.eof})
		socket := &usageSocket{raw: raw}
		c := &ProgressFiles{socket: socket, selector: "id:original", relative: ".steerrunner/progress/fixed/progress.bin"}
		data, err := c.Read(context.Background())
		if (err == nil) != tc.valid {
			t.Fatal(tc, err)
		}
		if tc.valid && len(data) != tc.count {
			t.Fatal(data)
		}
		if socket.method != "files.readChunk" {
			t.Fatal(socket.method)
		}
	}
}

func TestResultFileRPCUsesOriginalSelectorAndIndependentPath(t *testing.T) {
	socket := &usageSocket{raw: json.RawMessage(`{"contentBase64":"AP8=","bytesRead":2,"eof":true}`)}
	c := &ProgressFiles{socket: socket, selector: "id:original-worktree", relative: ".steerrunner/result/original-run/result.bin"}
	if err := c.Initialize(context.Background(), []byte{0, 255}); err != nil {
		t.Fatal(err)
	}
	params := socket.params.(map[string]any)
	if socket.method != "files.writeBase64" || params["worktree"] != c.selector || params["relativePath"] != c.relative || params["contentBase64"] != "AP8=" {
		t.Fatal(params)
	}
	if _, err := c.Read(context.Background()); err != nil {
		t.Fatal(err)
	}
	params = socket.params.(map[string]any)
	if params["worktree"] != c.selector || params["relativePath"] != c.relative || params["length"] != 65537 {
		t.Fatal(params)
	}
	// All invalid routes fail before connecting; no local path fallback exists.
	if _, err := OpenResultFiles(context.Background(), "", "runtime", "remote", "worktree", "C:/same-path", "instance", strings.Repeat("a", 32)); err == nil {
		t.Fatal("wrong host accepted")
	}
	if _, err := OpenResultFiles(context.Background(), "", "runtime", "local", "worktree", "C:/same-path", "instance", "../other-run"); err == nil {
		t.Fatal("foreign run accepted")
	}
}
