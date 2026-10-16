package livetest

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFailedObservationRequiresLaterOriginalRead(t *testing.T) {
	at := time.Now().UTC()
	entry := func(id, kind, terminal string, offset, ended int, failed bool) []byte {
		raw, _ := json.Marshal(map[string]any{"call_id": id, "kind": kind, "args": []string{"terminal", "show", "--terminal", terminal}, "at": at.Add(time.Duration(offset) * time.Second), "ended_at": at.Add(time.Duration(ended) * time.Second), "failed": failed})
		return append(raw, '\n')
	}
	bad := entry("old", "receipt", "original", -10, 1, true)
	bad = append(bad, entry("pending", "intent", "original", 2, 2, false)...)
	if FailedObservationRetried(bad, "original", at) {
		t.Fatal("pre-fault read accepted")
	}
	raw := entry("first", "receipt", "original", 1, 5, true)
	if FailedObservationRetried(append(append([]byte{}, raw...), entry("retry", "intent", "original", 6, 6, false)...), "original", at) {
		t.Fatal("orphan failed receipt accepted")
	}
	raw = append(entry("first", "intent", "original", 1, 1, false), raw...)
	for _, next := range [][]byte{entry("first", "intent", "original", 6, 6, false), entry("parallel", "intent", "original", 2, 2, false), entry("foreign", "intent", "replacement", 6, 6, false)} {
		if FailedObservationRetried(append(append([]byte{}, raw...), next...), "original", at) {
			t.Fatal("same, overlapping or foreign read accepted")
		}
	}
	raw = append(raw, entry("retry", "intent", "original", 6, 6, false)...)
	if !FailedObservationRetried(raw, "original", at) {
		t.Fatal("actual later retry rejected")
	}
}
