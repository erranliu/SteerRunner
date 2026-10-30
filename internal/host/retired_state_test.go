package host

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyStoppedRejectsRetiredStateWithoutHidingResources(t *testing.T) {
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "service"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "service", "writer.lock"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "runner.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(fmt.Sprintf(`CREATE TABLE metadata(format TEXT); INSERT INTO metadata VALUES('steerrunner-state/v%d'); PRAGMA user_version=%d; CREATE TABLE checks(id TEXT, body TEXT); INSERT INTO checks VALUES('old-active','{"status":"running","cleanup_status":"pending"}');`, version, version))
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
			result, fault := VerifyStopped(context.Background(), dir)
			if fault == nil || fault.Detail.Code != "unsupported_schema" || result.ResourcesSettled {
				t.Fatal("old resources hidden by successful empty report", result, fault)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("verification changed old state", err)
			}
		})
	}
}
