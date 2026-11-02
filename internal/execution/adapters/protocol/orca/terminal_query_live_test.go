package orca

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// The caller owns shell creation and exact cleanup. This opt-in test only reads
// an explicit terminal twice, using two independent production RPC connections.
func TestOrcaTerminalQueryLive(t *testing.T) {
	path := os.Getenv("STEERRUNNER_ORCA_TERMINAL_QUERY_TARGET")
	if path == "" {
		t.Skip("explicit owned Orca shell query target required")
	}
	var target struct {
		Environment, Worktree string
		Identity              TerminalIdentity
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &target) != nil || !strings.HasPrefix(target.Worktree, "id:") || !strings.Contains(target.Worktree, "::") {
		t.Fatal("invalid explicit terminal query target", err)
	}
	result := struct {
		Queries []Terminal
		Errors  []string
	}{}
	defer func() {
		raw, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		out, err := os.OpenFile(path+".result.json", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Error("query evidence already exists; use a new target file", err)
			return
		}
		defer out.Close()
		if _, err := out.Write(raw); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		terminal, err := QueryBoundTerminal(ctx, target.Environment, target.Identity)
		result.Queries = append(result.Queries, terminal)
		diagnostic := ""
		if err != nil {
			diagnostic = err.Error()
		}
		result.Errors = append(result.Errors, diagnostic)
		if err != nil || terminal.WorktreeID != strings.TrimPrefix(target.Worktree, "id:") || !terminal.Connected || terminal.Exited() {
			t.Fatal("original shell query unverified", terminal, err)
		}
	}
}
