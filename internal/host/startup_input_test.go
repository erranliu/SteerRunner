package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestPendingStopsRetainOriginalRequestsAcrossUnknownStartup(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	input := &contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: "run", Request: contracts.Request{Protocol: contracts.Protocol, RequestID: "original-stop", Input: json.RawMessage(`{"expected_revision":3}`)}}}}
	if err := savePendingStart(dir, input); err != nil {
		t.Fatal(err)
	}
	if err := savePendingStart(dir, nil); err != nil {
		t.Fatal(err)
	}
	saved, err := loadPendingStart(dir)
	if err != nil || len(saved.PendingStops) != 1 || saved.PendingStops[0].Request.RequestID != "original-stop" {
		t.Fatalf("lost cancellation: %+v %v", saved, err)
	}
	if err = savePendingStart(dir, &contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{}}); err == nil {
		t.Fatal("unresolved stop dropped")
	}
	input.PendingStops[0].Request.Input = json.RawMessage(`{"expected_revision":4}`)
	if err = savePendingStart(dir, input); err == nil {
		t.Fatal("unresolved stop changed")
	}
}
