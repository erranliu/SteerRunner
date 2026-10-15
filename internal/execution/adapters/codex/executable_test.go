package codexworker

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAppServerLaunchBindsNativeExecutable(t *testing.T) {
	path, err := NativeExecutable(os.Args[0])
	if err != nil || !isNativeCodexExecutable(path) {
		t.Fatalf("Go test executable was not recognized as a direct process: %q %v", path, err)
	}
	// L1 must not depend on a separately installed Codex package. Exercise the
	// Windows npm layout with a native header, without launching a real agent.
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(root, "codex.cmd")
	if err := os.WriteFile(shim, []byte(`@node "%~dp0\node_modules\@openai\codex\bin\codex.js" %*`), 0600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "node_modules", "@openai", "codex", "vendor", "x86_64-pc-windows-msvc", "bin", "codex.exe")
	if err := os.MkdirAll(filepath.Dir(want), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte{'M', 'Z', 0, 0}, 0600); err != nil {
		t.Fatal(err)
	}
	native, err := NativeExecutable(shim)
	if err != nil || native != want {
		t.Fatalf("npm wrapper did not resolve the native process: %q %v", native, err)
	}
}
