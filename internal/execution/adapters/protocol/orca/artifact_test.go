package orca

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestStopArtifactRequiresCompleteBoundedBytes(t *testing.T) {
	for _, mode := range []string{"complete", "truncated", "missing-eof", "missing-count", "wrong-count", "oversized", "invalid-base64"} {
		t.Run(mode, func(t *testing.T) {
			value := map[string]any{"contentBase64": base64.StdEncoding.EncodeToString([]byte("proof")), "bytesRead": 5, "eof": true}
			switch mode {
			case "truncated":
				value["eof"] = false
			case "missing-eof":
				delete(value, "eof")
			case "missing-count":
				delete(value, "bytesRead")
			case "wrong-count":
				value["bytesRead"] = 6
			case "oversized":
				value["bytesRead"] = 65537
			case "invalid-base64":
				value["contentBase64"] = "???"
			}
			raw, _ := json.Marshal(value)
			body, err := decodeStopArtifact(raw)
			if (err == nil) != (mode == "complete") || err == nil && string(body) != "proof" {
				t.Fatal(string(body), err)
			}
		})
	}
}
