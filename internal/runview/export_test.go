package runview

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestExportHeartbeatFailureAndUnknownReplacement(t *testing.T) {
	at := time.Now().UTC()
	var output []Snapshot
	fail := false
	runs := []Run{{ID: "b", Title: "title", Workflow: Workflow{"development", 2}, Status: "running"}, {ID: "a", Status: "running"}}
	e := Exporter{Directory: t.TempDir(), Source: strings.Repeat("a", 64), Instance: "original", Read: func() ([]Run, error) { return append([]Run(nil), runs...), nil }, Publish: func(_ context.Context, _ string, b []byte) error {
		var s Snapshot
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		output = append(output, s)
		if fail {
			return errors.New("replacement may have succeeded")
		}
		return nil
	}}
	for _, offset := range []time.Duration{0, time.Second, 5 * time.Second} {
		if err := e.Tick(context.Background(), at.Add(offset)); err != nil {
			t.Fatal(err)
		}
	}
	if len(output) != 2 || output[0].Runs[0].ID != "a" || output[1].Sequence != 2 {
		t.Fatal(output)
	}
	fail = true
	runs[0].Status = "blocked"
	if e.Tick(context.Background(), at.Add(6*time.Second)) == nil {
		t.Fatal("fault lost")
	}
	fail = false
	_ = e.Tick(context.Background(), at.Add(7*time.Second))
	if len(output) != 3 {
		t.Fatal("failure backoff ignored")
	}
	if err := e.Tick(context.Background(), at.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if output[3].Sequence != 4 || !output[3].Generated.Equal(at.Add(11*time.Second)) {
		t.Fatal("unknown attempt reused sequence or changed sampling time", output)
	}
	e.Read = func() ([]Run, error) { return nil, errors.New("incomplete source") }
	_ = e.Tick(context.Background(), at.Add(time.Minute))
	if len(output) != 4 {
		t.Fatal("read failure published fake empty/heartbeat")
	}
}
func TestPublishAtomicReaderSharingAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.json")
	if err := Publish(context.Background(), path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(path)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	if err = Publish(context.Background(), path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Publish(ctx, path, []byte("cancelled")) == nil {
		t.Fatal("cancelled publish succeeded")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "new" {
		t.Fatal(string(b))
	}
	blocked := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocked, []byte("keep"), 0600)
	if Publish(context.Background(), filepath.Join(blocked, "invalid.json"), []byte("bad")) == nil {
		t.Fatal("invalid destination accepted")
	}
}
func TestExportStopWaitsForOriginalPublisher(t *testing.T) {
	entered := make(chan struct{})
	settle := make(chan struct{})
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	e := Exporter{Source: "source", Instance: "original", Read: func() ([]Run, error) { return nil, nil }, Publish: func(ctx context.Context, _ string, _ []byte) error {
		close(entered)
		<-ctx.Done()
		<-settle
		return ctx.Err()
	}}
	go func() { defer close(done); e.Run(ctx, func(error) {}) }()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("writer escaped before original I/O settled")
	default:
	}
	close(settle)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("exporter did not settle")
	}
}
