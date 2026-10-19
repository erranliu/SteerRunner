package livetest

import (
	"bytes"
	"testing"
)

func TestNativeAuditStoppedProbeBoundary(t *testing.T) {
	probe := []byte("{\"run\":\"probe\",\"stopped\":true}\n")
	main := []byte("{\"run\":\"main\"}\n")
	raw := append(append([]byte(nil), probe...), main...)
	got, err := NativeAuditAfterProbe(raw, probe)
	if err != nil || !bytes.Equal(got, main) {
		t.Fatal("original stopped probe did not preserve the main journal", err)
	}
	for _, test := range []struct {
		name        string
		raw, prefix []byte
	}{
		{"missing", raw, nil},
		{"truncated", probe[:len(probe)-1], probe},
		{"inside_record", raw, probe[:len(probe)-1]},
		{"rewritten_prefix", append([]byte("different\n"), main...), probe},
		{"no_main", probe, probe},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NativeAuditAfterProbe(test.raw, test.prefix); err == nil {
				t.Fatal("unbound journal boundary accepted")
			}
		})
	}
}
