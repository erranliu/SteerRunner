package storage

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredStateFormatsRefusedWithoutChangingOriginalResources(t *testing.T) {
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "runner.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			// An active old resource must never disappear behind a successful
			// current-format empty resource report. No old decoder is used.
			_, err = db.Exec(fmt.Sprintf(`CREATE TABLE metadata(format TEXT); INSERT INTO metadata VALUES('steerrunner-state/v%d'); PRAGMA user_version=%d; CREATE TABLE checks(id TEXT, body TEXT); INSERT INTO checks VALUES('owned-old-resource','{"status":"running","cleanup_status":"pending"}');`, version, version))
			if err != nil {
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, open := range []func(string) (*Store, error){Open, OpenReader} {
				store, err := open(dir)
				if err == nil {
					store.Close()
					t.Fatal("retired state accepted")
				}
				var fault *contracts.Fault
				if !errors.As(err, &fault) || fault.Detail.Code != "unsupported_schema" {
					t.Fatal("wrong refusal reason", err)
				}
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal altered original state", err)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatal("refusal created migration/backup files", err)
			}
		})
	}
}

func TestFreshStateHasOnlySupportedRunResources(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var count, version int
	var format string
	if err = s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='checks'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("retired table", count, err)
	}
	if err = s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('commands') WHERE name='check_id'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("retired command index", count, err)
	}
	if s.db.QueryRow(`PRAGMA user_version`).Scan(&version) != nil || version != 5 || s.db.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || format != Format {
		t.Fatal("format disagreement")
	}
}
