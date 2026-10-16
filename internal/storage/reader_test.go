package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSROfflineReaderDoesNotCreateOrWrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	if r, err := OpenReader(dir); err == nil {
		r.Close()
		t.Fatal("missing state accepted")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("reader created state", err)
	}
	dir = t.TempDir()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "runner.db")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := OpenReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("DELETE FROM metadata"); err == nil {
		t.Fatal("reader allowed a write")
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("reader changed storage", err)
	}
}
