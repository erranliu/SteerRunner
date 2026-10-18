package storage

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"testing"
)

func TestRAVersion(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE metadata SET format='steerrunner-state/v2'; PRAGMA user_version=2;"); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if opened, err := Open(dir); err == nil {
		_ = opened.Close()
		t.Fatal("v2 storage was silently upgraded")
	} else {
		var f *contracts.Fault
		if !errors.As(err, &f) || f.Detail.Code != "unsupported_schema" {
			t.Fatal(err)
		}
	}
	if opened, err := OpenReader(dir); err == nil {
		_ = opened.Close()
		t.Fatal("v2 reader accepted")
	}
}
