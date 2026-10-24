package livetest

import (
	"errors"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// ProgressObservation checks a waiting producer without assuming an infallible
// transport. A finite collection gap must retain the entire original snapshot
// and recover before its budget; terminal/binding failures are never accepted.
type ProgressObservation struct {
	snapshot contracts.ProgressFileView
	maxGap   time.Duration
	gapSince time.Time
	healthy  bool
}

func NewProgressObservation(snapshot contracts.ProgressFileView, maxGap time.Duration) *ProgressObservation {
	if snapshot.ContentBase64 != nil {
		content := *snapshot.ContentBase64
		snapshot.ContentBase64 = &content
	}
	if snapshot.ObservedAt != nil {
		at := *snapshot.ObservedAt
		snapshot.ObservedAt = &at
	}
	return &ProgressObservation{snapshot: snapshot, maxGap: maxGap}
}

func (o *ProgressObservation) Sample(at time.Time, current contracts.ProgressFileView) error {
	want := o.snapshot
	if o.maxGap <= 0 || want.Version == 0 || want.ContentBase64 == nil || want.ObservedAt == nil ||
		current.RunID != want.RunID || current.Contract != want.Contract || current.Encoding != want.Encoding ||
		current.Version != want.Version || current.ContentBase64 == nil || *current.ContentBase64 != *want.ContentBase64 ||
		current.SizeBytes != want.SizeBytes || current.SHA256 != want.SHA256 || current.ObservedAt == nil || !current.ObservedAt.Equal(*want.ObservedAt) {
		return errors.New("waiting producer snapshot changed")
	}
	if !o.gapSince.IsZero() && at.Sub(o.gapSince) > o.maxGap {
		return errors.New("waiting producer collection did not recover within its budget")
	}
	switch {
	case current.CollectionStatus == "observing" && current.Diagnostic == "":
		o.gapSince, o.healthy = time.Time{}, true
	case current.CollectionStatus == "unavailable" && (current.Diagnostic == "transport_unavailable" || current.Diagnostic == "read_failed"):
		if o.gapSince.IsZero() {
			o.gapSince = at
		}
		o.healthy = false
	default:
		return errors.New("waiting producer collection has an unexpected status or diagnostic")
	}
	return nil
}

func (o *ProgressObservation) Healthy() bool { return o.healthy }
