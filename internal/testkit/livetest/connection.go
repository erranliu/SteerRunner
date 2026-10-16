package livetest

import (
	"bytes"
	"encoding/json"
	"time"
)

type ConnectionProof struct {
	StartedAt  time.Time `json:"started_at"`
	ObservedAt time.Time `json:"observed_at"`
	Terminal   string    `json:"terminal"`
}

// FailedObservationRetried requires a real failed read of the original terminal,
// followed by a different read started after that failure. Probe commands and
// reads started before the network fault cannot satisfy the barrier.
func FailedObservationRetried(raw []byte, terminal string, after time.Time) bool {
	type call struct {
		Kind   string    `json:"kind"`
		ID     string    `json:"call_id"`
		At     time.Time `json:"at"`
		Ended  time.Time `json:"ended_at"`
		Args   []string  `json:"args"`
		Failed bool      `json:"failed"`
	}
	var failed []call
	var starts []call
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var c call
		if json.Unmarshal(line, &c) != nil || c.ID == "" || c.At.Before(after) || len(c.Args) < 4 || c.Args[0] != "terminal" || c.Args[1] != "show" {
			continue
		}
		bound := false
		for i := 2; i+1 < len(c.Args); i++ {
			if c.Args[i] == "--terminal" && c.Args[i+1] == terminal {
				bound = true
			}
		}
		if !bound {
			continue
		}
		if c.Kind == "receipt" && c.Failed && !c.Ended.Before(c.At) {
			failed = append(failed, c)
		}
		if c.Kind == "intent" {
			starts = append(starts, c)
		}
	}
	for _, failure := range failed {
		matched := false
		for _, start := range starts {
			if start.ID == failure.ID && start.At.Equal(failure.At) {
				matched = true
			}
		}
		if !matched {
			continue
		}
		for _, start := range starts {
			if start.ID != failure.ID && !start.At.Before(failure.Ended) {
				return true
			}
		}
	}
	return false
}
