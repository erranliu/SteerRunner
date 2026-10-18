package resultacquisition

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRASuccessOutput(t *testing.T) {
	marker := "实施及必要验证已完成。"
	for name, body := range map[string]string{"bare": marker, "final": "details\n" + marker + "\r\n\r\n", "closed fence": "```json\n{}\n```\n" + marker, "tilde": "~~~\na\n~~~\n" + marker} {
		t.Run(name, func(t *testing.T) {
			if !RecognizeSuccessOutput(body, marker) {
				t.Fatal(body)
			}
		})
	}
	for name, body := range map[string]string{"quote": "> " + marker, "indent": "    " + marker, "tab": "\t" + marker, "inline": "done " + marker, "substring": marker + " extra", "not-last": marker + "\nquestion?", "open-fence": "```text\n" + marker, "long-fence": "````\na\n```\n" + marker, "tilde-open": "~~~\n" + marker, "space": marker + " ", "echo": "write the following line: " + marker} {
		t.Run(name, func(t *testing.T) {
			if RecognizeSuccessOutput(body, marker) {
				t.Fatal(body)
			}
		})
	}
}
func TestRAContract(t *testing.T) {
	good := `{"outcome":"done","summary":"checked","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`
	if v, err := DecodeReport(good, "audit"); err != nil || v.FixedCount == nil || *v.FixedCount != 0 {
		t.Fatal(v, err)
	}
	for name, body := range map[string]string{"alias": strings.Replace(good, "fixed_count", "Fixed_Count", 1), "duplicate": strings.Replace(good, `"fixed_count":0`, `"fixed_count":0,"fixed_count":1`, 1), "count": strings.Replace(good, `"fixed_count":0`, `"fixed_count":1`, 1), "null": strings.Replace(good, `"fixed_issues":[]`, `"fixed_issues":null`, 1), "missing": `{"outcome":"done","summary":"checked"}`, "old": `{"outcome":"done","summary":"checked","verdict":"pass","findings":[]}`, "unknown": `{"outcome":"unknown","summary":"missing"}`, "trailing": good + ` {}`, "bad_severity": `{"outcome":"done","summary":"checked","fixed_count":1,"fixed_issues":[{"severity":"P3","issue":"a","repair":"b","evidence":"c"}],"remaining_issues":[]}`, "remaining": `{"outcome":"done","summary":"checked","fixed_count":0,"fixed_issues":[],"remaining_issues":[{"severity":"P2","issue":"a","reason":"b"}]}`} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReport(body, "audit"); err == nil {
				t.Fatal(body)
			}
		})
	}
}
func TestRAConfig(t *testing.T) {
	dir := t.TempDir()
	f, err := LoadConfig(dir)
	if err != nil || f.Config.CodexParser.Enabled || f.Config.CodexParser.TimeoutSeconds != 120 {
		t.Fatal(f, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "service", "result-acquisition.json")
	for name, body := range map[string]string{"unknown": `{"unknown":true}`, "null": `{"codex_parser":null}`, "duplicate": `{"codex_parser":{"enabled":false,"enabled":true}}`, "range": `{"codex_parser":{"timeout_seconds":301}}`, "alias": `{"codex_parser":{"Enabled":true}}`, "home": `{"codex_parser":{"codex_home_env":"bad name"}}`} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(dir); err == nil {
				t.Fatal(body)
			}
		})
	}
	if err := os.WriteFile(path, []byte{123, 34, 255, 34, 58, 48, 125}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("invalid UTF-8 config accepted")
	}
	c := DefaultConfig()
	c.CodexParser.Enabled = true
	c.CodexParser.Executable = "missing-codex.exe"
	raw, _ := json.Marshal(c)
	_ = os.WriteFile(path, raw, 0600)
	f, err = LoadConfig(dir)
	if err != nil || !f.Config.CodexParser.Enabled || f.FileDigest != Digest(raw) {
		t.Fatal(f, err)
	}
}
func TestRAParserQualification(t *testing.T) {
	if supported, reason := ParserSupport(); !supported || !strings.Contains(reason, "not an empty-tool guarantee") {
		t.Fatal("minimal profile boundary is missing", reason)
	}
}
func TestRASelectionPRClues(t *testing.T) {
	urls := PRURLs("[PR](https://example.test/o/r/pull/1). https://example.test/o/r/pull/1 https://example.test/o/r/pulls/2")
	if len(urls) != 2 {
		t.Fatal(urls)
	}
	if got := PRURLs("https://secret@example.test/o/r/pull/1 https://example.test/o/r/pull/1?secret=x"); len(got) != 0 {
		t.Fatal(got)
	}
}

// The live publication final used its URL as the Markdown label as well as
// the destination. Each clue must remain an independently verifiable URL.
func TestRAPRCluesRespectMarkdownBoundaries(t *testing.T) {
	const first = "https://github.com/erranliu/SteerlineTest/pull/466"
	const second = "https://github.com/erranliu/SteerlineTest/pull/467"
	for _, text := range []string{
		"Published: [" + first + "](" + first + ")\n本次发布已完成。",
		"Published: <" + first + ">.",
		"Published: (" + first + "),",
		"Published: `" + first + "`.",
	} {
		got := PRURLs(text)
		if len(got) != 1 || got[0] != first {
			t.Fatalf("%q: %v", text, got)
		}
	}
	got := PRURLs("[" + first + "](" + second + ")")
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatal("different label/destination must retain ambiguity", got)
	}
}
