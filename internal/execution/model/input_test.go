package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestInputBudgetBoundaries(t *testing.T) {
	for _, pair := range [][2]int{{1999, 0}, {2000, 0}, {2001, 0}, {2999, 0}, {3000, 0}, {3001, 0}, {10552, 0}, {1 << 20, 1000}, {0, 999}, {0, 1000}, {0, 1001}, {1999, 1000}, {2000, 1000}, {2001, 1000}, {2000, 1001}, {10552, 1001}} {
		text, b, err := FreezeInput(InputPart{"caller.direct", CallerInput, strings.Repeat("c", pair[0])}, InputPart{"native.header", RunnerInput, strings.Repeat("r", pair[1])})
		want := pair[1] > RunnerInputBytes
		var limit *InputBudgetError
		if errors.As(err, &limit) != want || b.CallerBytes != pair[0] || b.RunnerBytes != pair[1] || b.TotalBytes != len(text) || b.TotalBytes != pair[0]+pair[1] {
			t.Fatalf("%v: %+v %v", pair, b, err)
		}
		if want && (!strings.Contains(err.Error(), "caller=") || !strings.Contains(err.Error(), "runner=") || !strings.Contains(err.Error(), "total=")) {
			t.Fatal(err)
		}
	}
}
func TestInputFreezeAndNativeNewlines(t *testing.T) {
	parts := []InputPart{{"caller.direct", CallerInput, "中文😀e\u0301\r"}, {"native.header", RunnerInput, "\nlabel\r\n"}, {"caller.feedback", CallerInput, "原文\n\n"}}
	text, b, err := FreezeInput(NativeInputParts(parts)...)
	if err != nil || text != "中文😀e\u0301\nlabel\n原文" || b.CallerBytes != len("中文😀e\u0301原文") || b.RunnerBytes != len("\nlabel\n") {
		t.Fatalf("%q %+v %v", text, b, err)
	}
	if CheckInput(text+"!", b) == nil {
		t.Fatal("undeclared append accepted")
	}
	changed := b.Clone()
	changed.Fragments[0].Source = "workflow.body"
	if CheckInput(text, changed) == nil {
		t.Fatal("changed provenance accepted")
	}
	if _, _, err := FreezeInput(InputPart{"undeclared", RunnerInput, "instruction"}); err == nil {
		t.Fatal("undeclared source accepted")
	}
	if _, _, err := FreezeInput(InputPart{"caller.direct", CallerInput, string([]byte{0xff})}); err == nil {
		t.Fatal("invalid UTF8 accepted")
	}
}
func TestAnswerTextExactNativeJSON(t *testing.T) {
	for _, answers := range []map[string][]string{nil, {}, {"问😀": {"中文\n\"<&😀", "e\u0301"}, "a": nil}, {"q": {strings.Repeat("中文😀\n", 1000)}}} {
		text, b, err := FreezeInput(AnswerParts(answers)...)
		expected, _ := json.Marshal(map[string]any{"answers": answers})
		if err != nil || text != string(expected) || b.TotalBytes != len(expected) {
			t.Fatalf("%s != %s: %v", text, expected, err)
		}
	}
}
