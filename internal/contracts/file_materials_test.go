package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateFileMaterialContract(t *testing.T) {
	for _, paths := range [][]string{{"inputs/source.md", "inputs/graph.json"}, {"../escape"}, {"C:/escape"}, {"CON.txt"}, {"a", "A/x"}, {"a", "A"}, {"a/../b"}, {".git/config"}, {"a. /b"}, {"a\nb"}} {
		body := "fixed\r\nJSON"
		request := RunRequest{Title: "files", Workflow: Workflow{ID: "generate", Version: 1}, Execution: Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"read_only"}`)}, Generate: &GenerateInput{Instruction: "Read files", Materials: []Document{}}}
		for _, path := range paths {
			request.Generate.Materials = append(request.Generate.Materials, Document{Kind: "file", Path: path, Content: &body})
		}
		raw, _ := json.Marshal(request)
		if fault := ValidateRun(raw); (fault == nil) != (paths[0] == "inputs/source.md") {
			t.Fatal(paths, fault)
		}
	}
	if ValidFileMaterialPath("inputs/" + strings.Repeat("x", 1024)) {
		t.Fatal("unbounded file path accepted")
	}
}
