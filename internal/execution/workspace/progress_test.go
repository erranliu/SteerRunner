package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProgressFileBytesAndAtomicReplacement(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	root := t.TempDir()
	id := "01234567890123456789012345678901"
	dirs, err := ProgressDirectories(root, id, true)
	if err != nil {
		t.Fatal(err)
	}
	initial := []byte{0, 255, 1, 'x'}
	if err := InitializeProgress(root, id, dirs, initial); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, filepath.FromSlash(ProgressRelative(id)))
	for _, data := range [][]byte{initial, []byte("not json"), {}, bytes.Repeat([]byte{3}, 65536)} {
		temp := file + ".next"
		if err := os.WriteFile(temp, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(temp, file); err != nil {
			t.Fatal(err)
		}
		got, err := ReadProgress(root, id, dirs)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal(len(got), err)
		}
	}
	if err := InitializeProgress(root, id, dirs, initial); err != nil {
		t.Fatal(err)
	}
	got, err := ReadProgress(root, id, dirs)
	if err != nil || len(got) != 65536 {
		t.Fatal("initialization clobbered", err)
	}
	_ = os.WriteFile(file, bytes.Repeat([]byte{1}, 65537), 0600)
	if _, err := ReadProgress(root, id, dirs); err == nil || err.Error() != "size_exceeded" {
		t.Fatal(err)
	}
	_ = os.Remove(file)
	if _, err := ReadProgress(root, id, dirs); err == nil || err.Error() != "file_missing" {
		t.Fatal(err)
	}
	dirs["."] = "different"
	if _, err := ReadProgress(root, id, dirs); err == nil {
		t.Fatal("wrong binding accepted")
	}
}
