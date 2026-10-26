package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProgressFileBytesAndAtomicReplacement(t *testing.T) { testManagedFileBytes(t, false) }
func TestResultFileBytesAndAtomicReplacement(t *testing.T)   { testManagedFileBytes(t, true) }
func testManagedFileBytes(t *testing.T, result bool) {
	directories, initialize, read, relative := ProgressDirectories, InitializeProgress, ReadProgress, ProgressRelative
	if result {
		directories, initialize, read, relative = ResultDirectories, InitializeResult, ReadResult, ResultRelative
	}

	if runtime.GOOS != "windows" {
		return
	}
	root := t.TempDir()
	id := "01234567890123456789012345678901"
	dirs, err := directories(root, id, true)
	if err != nil {
		t.Fatal(err)
	}
	initial := []byte{0, 255, 1, 'x'}
	if err := initialize(root, id, dirs, initial); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, filepath.FromSlash(relative(id)))
	for _, data := range [][]byte{initial, []byte("not json"), {}, bytes.Repeat([]byte{3}, 65536)} {
		temp := file + ".next"
		if err := os.WriteFile(temp, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(temp, file); err != nil {
			t.Fatal(err)
		}
		got, err := read(root, id, dirs)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal(len(got), err)
		}
	}
	if err := initialize(root, id, dirs, initial); err != nil {
		t.Fatal(err)
	}
	got, err := read(root, id, dirs)
	if err != nil || len(got) != 65536 {
		t.Fatal("initialization clobbered", err)
	}
	_ = os.WriteFile(file, bytes.Repeat([]byte{1}, 65537), 0600)
	if _, err := read(root, id, dirs); err == nil || err.Error() != "size_exceeded" {
		t.Fatal(err)
	}
	_ = os.Remove(file)
	if _, err := read(root, id, dirs); err == nil || err.Error() != "file_missing" {
		t.Fatal(err)
	}
	dirs["."] = "different"
	if _, err := read(root, id, dirs); err == nil {
		t.Fatal("wrong binding accepted")
	}
}
