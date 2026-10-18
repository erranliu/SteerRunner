package livetest

import (
	"strings"
	"testing"
)

func TestNativeInputFingerprintRejectsEmptyAndTracksOriginalSends(t *testing.T) {
	send := `{"kind":"intent","call_id":"1","args":["terminal","send","--terminal","original","--text","first"]}` + "\n"
	for _, raw := range []string{"", `{"kind":"receipt","call_id":"1","args":["terminal","send","--terminal","original"]}`, strings.ReplaceAll(send, "original", "peer"), "invalid"} {
		if _, e := TerminalInputFingerprint([]byte(raw), "original"); e == nil {
			t.Fatal("empty/unrelated evidence passed", raw)
		}
	}
	a, e := TerminalInputFingerprint([]byte(send), "original")
	if e != nil {
		t.Fatal(e)
	}
	b, e := TerminalInputFingerprint([]byte(send+strings.ReplaceAll(send, "original", "peer")), "original")
	if e != nil || a != b {
		t.Fatal("peer contaminated original inputs", e)
	}
	c, e := TerminalInputFingerprint([]byte(send+strings.ReplaceAll(send, "first", "second")), "original")
	if e != nil || c == a {
		t.Fatal("additional send was invisible", e)
	}
}
