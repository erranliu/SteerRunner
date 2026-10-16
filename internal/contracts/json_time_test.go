package contracts

import (
	"testing"
	"time"
)

func TestStrictJSONNativeDeadline(t *testing.T) {
	var value struct {
		Deadline time.Time `json:"deadline"`
	}
	if err := DecodeStrict([]byte(`{"deadline":"2026-10-16T10:00:00.123Z"}`), &value); err != nil || value.Deadline.Nanosecond() != 123000000 {
		t.Fatal("native RFC3339 deadline rejected", err)
	}
	for _, raw := range []string{`{"deadline":{}}`, `{"deadline":null}`, `{"deadline":"invalid"}`, `{"Deadline":"2026-10-16T10:00:00Z"}`, `{"deadline":"2026-10-16T10:00:00Z","deadline":"2026-10-16T10:00:01Z"}`} {
		if err := DecodeStrict([]byte(raw), &value); err == nil {
			t.Fatalf("invalid or ambiguous deadline admitted: %s", raw)
		}
	}
}
