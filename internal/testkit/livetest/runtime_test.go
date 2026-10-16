package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFrozenNativeRuntimeRejectsChangedMissingAndEscapingFiles(t *testing.T) {
	root := t.TempDir()
	runtimeRoot := filepath.Join(root, "runtime")
	_ = os.Mkdir(runtimeRoot, 0700)
	exe := filepath.Join(runtimeRoot, "node.exe")
	_ = os.WriteFile(exe, []byte("original public runtime"), 0600)
	sum := sha256.Sum256([]byte("original public runtime"))
	indexPath := filepath.Join(root, "runtime-files.json")
	m := Manifest{Stability: &StabilityConfig{NativeOrca: Tool{Path: exe}, RuntimeFiles: Tool{Path: indexPath}}}
	write := func(files []Tool) {
		raw, _ := json.Marshal(map[string]any{"schema": "steerrunner-native-runtime/v1", "files": files})
		_ = os.WriteFile(indexPath, raw, 0600)
	}
	files := []Tool{{Path: "node.exe", SHA256: hex.EncodeToString(sum[:])}}
	write(files)
	if err := ValidateRuntime(m); err != nil {
		t.Fatal("valid generated runtime index rejected", err)
	}
	extra := filepath.Join(runtimeRoot, "unexpected.js")
	_ = os.WriteFile(extra, []byte("unexpected module"), 0600)
	if ValidateRuntime(m) == nil {
		t.Fatal("unlisted runtime module accepted")
	}
	_ = os.Remove(extra)
	_ = os.WriteFile(exe, []byte("changed"), 0600)
	if ValidateRuntime(m) == nil {
		t.Fatal("changed public runtime accepted")
	}
	_ = os.Remove(exe)
	if ValidateRuntime(m) == nil {
		t.Fatal("missing public runtime accepted")
	}
	for _, path := range []string{"../outside.exe", exe} {
		write([]Tool{{Path: path, SHA256: files[0].SHA256}})
		if ValidateRuntime(m) == nil {
			t.Fatal("unsafe runtime path accepted", path)
		}
	}
	write(nil)
	if ValidateRuntime(m) == nil {
		t.Fatal("empty runtime index accepted")
	}
	_ = os.WriteFile(indexPath, []byte(`[]`), 0600)
	if ValidateRuntime(m) == nil {
		t.Fatal("legacy root array accepted")
	}
}
