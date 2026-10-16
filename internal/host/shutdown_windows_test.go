//go:build windows

package host

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRShutdownProofRequiresReleasedWriter(t *testing.T) {
	dir, err := Normalize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	lock, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), true)
	if err != nil || busy {
		t.Fatal(err)
	}
	proof := contracts.ServiceShutdown{InstanceID: "original", RequestID: "stop", StateDir: dir, ExecutableSHA256: "fixed", ResourcesSettled: true}
	if err = writeShutdown(dir, proof); err != nil {
		lock.Close()
		t.Fatal(err)
	}
	info, failure := Status(context.Background(), dir)
	if failure != nil || info.Status != "unavailable" || info.LastShutdown != nil {
		lock.Close()
		t.Fatal(info, failure)
	}
	if err = lock.Close(); err != nil {
		t.Fatal(err)
	}
	info, failure = Status(context.Background(), dir)
	if failure != nil || info.Status != "not_running" || info.LastShutdown == nil || *info.LastShutdown != proof {
		t.Fatal(info, failure)
	}
	if err = os.WriteFile(shutdownPath(dir), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, failure = Status(context.Background(), dir); failure == nil {
		t.Fatal("malformed proof was accepted")
	}
}
