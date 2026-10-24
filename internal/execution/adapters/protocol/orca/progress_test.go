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
