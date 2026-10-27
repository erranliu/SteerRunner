package orcatransport

import (
	"strings"
	"testing"
)

func TestNativePasteDraftSegments(t *testing.T) {
	for _, c := range []struct {
		name, draft, text string
		want              bool
	}{
		{"observed-two-elements", "[Pasted Content 2183 chars][Pasted Content 4233 chars]", strings.Repeat("x", 6416), true},
		{"different-boundaries", "[Pasted Content 2000 chars][Pasted Content 4416 chars]", strings.Repeat("x", 6416), true},
		{"duplicate-size-labels", "[Pasted Content 2 chars][Pasted Content 2 chars] #2[Pasted Content 2 chars] #3", "abcdef", true},
		{"visible-fragments", "head [Pasted Content 2 chars]\n\n[Pasted Content 3 chars] tail", "head 中文\n\nabc tail", true},
		{"plain", "literal [Pasted Content 2 chars]", "literal [Pasted Content 2 chars]", true},
		{"non-bmp-count-retained", "[Pasted Content 2 chars]", "中🙂", true},
		{"wrong-total", "[Pasted Content 2183 chars][Pasted Content 4232 chars]", strings.Repeat("x", 6416), false},
		{"over-total", "[Pasted Content 2183 chars][Pasted Content 4234 chars]", strings.Repeat("x", 6416), false},
		{"wrong-visible-fragment", "head [Pasted Content 2 chars] WRONG", "head 中文 tail", false},
		{"missing-ordinal", "[Pasted Content 2 chars][Pasted Content 2 chars]", "abcd", false},
		{"invalid-first-ordinal", "[Pasted Content 2 chars] #1", "ab", false},
		{"foreign-ordinal", "[Pasted Content 2 chars] #2", "ab", false},
		{"skipped-ordinal", "[Pasted Content 2 chars][Pasted Content 2 chars] #3", "abcd", false},
		{"zero", "[Pasted Content 0 chars]", "", false},
		{"overflow", "[Pasted Content 999999999999999999999999999 chars]", "abc", false},
		{"trailing-draft", "[Pasted Content 3 chars]x", "abc", false},
		{"truncated", "[Pasted Content 3 chars", "abc", false},
		{"emoji-loss", "[Pasted Content 1 chars]", "中🙂", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := nativePasteDraftMatches(c.draft, c.text); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
