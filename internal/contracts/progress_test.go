package contracts

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestProgressInputOpaqueBytesAndSupportedRoutes(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		for _, kind := range []string{"local", "orca_environment"} {
			r := RunRequest{Execution: Execution{Adapter: adapter, Target: Target{Kind: kind}}, ProgressFile: &ProgressFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte{0, 255, '{', 'x'})}}
			if err := ValidateProgress(r); err != nil {
				t.Fatal(adapter, kind, err)
			}
			for _, content := range []string{"?", "YQ==\n", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", MaxProgressBytes+1)))} {
				r.ProgressFile.InitialContentBase64 = content
				if ValidateProgress(r) == nil {
					t.Fatal("invalid/oversized accepted")
				}
			}
		}
	}
	for _, raw := range []string{`{"initial_content_base64":"","path":"C:/foreign"}`, `{"initial_content_base64":"","steps":[]}`} {
		var p ProgressFileInput
		if DecodeStrict([]byte(raw), &p) == nil {
			t.Fatal("caller path/semantics accepted")
		}
	}
}
