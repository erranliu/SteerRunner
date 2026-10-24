package livetest

import (
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func progressObservationSample() contracts.ProgressFileView {
	at := time.Unix(100, 0).UTC()
	content := "YW55AP8="
	return contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: "original", Version: 2, ObservedAt: &at, Encoding: "base64", ContentBase64: &content, SizeBytes: 5, SHA256: "original-digest", CollectionStatus: "observing"}
}

func TestProgressObservationRecoversRetainedSnapshot(t *testing.T) {
	original := progressObservationSample()
	o := NewProgressObservation(original, 20*time.Second)
	at := time.Unix(200, 0)
	if err := o.Sample(at, original); err != nil || !o.Healthy() {
		t.Fatal("initial observation", err)
	}
	gap := original
	gap.CollectionStatus, gap.Diagnostic = "unavailable", "transport_unavailable"
	if err := o.Sample(at.Add(time.Second), gap); err != nil || o.Healthy() {
		t.Fatal("finite gap was rejected or reported healthy", err)
	}
	gap.Diagnostic = "read_failed"
	if err := o.Sample(at.Add(10*time.Second), gap); err != nil || o.Healthy() {
		t.Fatal("gap diagnostic reset its budget", err)
	}
	if err := o.Sample(at.Add(19*time.Second), original); err != nil || !o.Healthy() {
		t.Fatal("same snapshot did not recover", err)
	}
}

func TestProgressObservationRejectsPersistentOrLateRecovery(t *testing.T) {
	for _, recover := range []bool{false, true} {
		o := NewProgressObservation(progressObservationSample(), 20*time.Second)
		at := time.Unix(200, 0)
		gap := progressObservationSample()
		gap.CollectionStatus, gap.Diagnostic = "unavailable", "transport_unavailable"
		if err := o.Sample(at, gap); err != nil || o.Healthy() {
			t.Fatal(err)
		}
		if recover {
			gap.CollectionStatus, gap.Diagnostic = "observing", ""
		}
		if err := o.Sample(at.Add(21*time.Second), gap); err == nil {
			t.Fatal("persistent gap or late recovery passed", recover)
		}
	}
}

func TestProgressObservationRejectsChangedSnapshotDuringGap(t *testing.T) {
	for _, change := range []func(*contracts.ProgressFileView){
		func(v *contracts.ProgressFileView) { v.RunID = "another" },
		func(v *contracts.ProgressFileView) { v.Version++ },
		func(v *contracts.ProgressFileView) { content := "different"; v.ContentBase64 = &content },
		func(v *contracts.ProgressFileView) { v.SizeBytes++ },
		func(v *contracts.ProgressFileView) { v.SHA256 = "different" },
		func(v *contracts.ProgressFileView) { at := v.ObservedAt.Add(time.Second); v.ObservedAt = &at },
	} {
		o := NewProgressObservation(progressObservationSample(), 20*time.Second)
		v := progressObservationSample()
		v.CollectionStatus, v.Diagnostic = "unavailable", "transport_unavailable"
		change(&v)
		if err := o.Sample(time.Unix(200, 0), v); err == nil {
			t.Fatal("collection gap hid changed snapshot", v)
		}
	}
}

func TestProgressObservationRejectsBindingAndTerminalFailures(t *testing.T) {
	for _, state := range [][2]string{{"unavailable", "binding_invalid"}, {"unavailable", "size_exceeded"}, {"closed", ""}, {"detached", ""}, {"observing", "read_failed"}} {
		o := NewProgressObservation(progressObservationSample(), 20*time.Second)
		v := progressObservationSample()
		v.CollectionStatus, v.Diagnostic = state[0], state[1]
		if err := o.Sample(time.Unix(200, 0), v); err == nil {
			t.Fatal("unexpected collection state passed", state)
		}
	}
}
