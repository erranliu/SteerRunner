// native-input-audit is a read-only, independent original-input checker.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func main() {
	if len(os.Args) != 2 {
		panic("expected one frozen audit request file")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var request struct {
		Environment string            `json:"environment"`
		Process     b.ProcessIdentity `json:"process"`
		Pane        string            `json:"pane"`
		Baseline    int64             `json:"baseline"`
		Input       string            `json:"input"`
		Final       string            `json:"final"`
	}
	if err = json.Unmarshal(raw, &request); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	sample, err := native.QueryOriginalSample(ctx, request.Environment, request.Process, request.Pane)
	result := map[string]any{"passed": false, "sample": sample, "at": time.Now().UTC()}
	if err == nil {
		var original native.AgentState
		original, err = native.QueryOriginalInput(ctx, request.Environment, request.Process, request.Pane, request.Baseline, sample.AgentState)
		result["original"] = original
		result["original_input"] = original.NativeInput
		if original.NativeInput != nil {
			result["matches_process"] = original.NativeInput.MatchesOriginalProcess(request.Process)
			result["matches_query"] = original.NativeInput.MatchesOriginalQuery(original)
			result["matches_text"] = original.NativeInput.Text == request.Input
		}
		result["matches_final"] = strings.TrimSpace(original.LastAssistantMessage) == request.Final
		if err == nil && (original.NativeInput == nil || original.NativeInput.Text != request.Input || !original.NativeInput.MatchesOriginalProcess(request.Process) || !original.NativeInput.MatchesOriginalQuery(original) || strings.TrimSpace(original.LastAssistantMessage) != request.Final) {
			err = fmt.Errorf("full unique original input/process/completion mismatch")
		}
	}
	if err != nil {
		result["error"] = err.Error()
	} else {
		result["passed"] = true
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		panic(err)
	}
	if err != nil {
		os.Exit(1)
	}
}
