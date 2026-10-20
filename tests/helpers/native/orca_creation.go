package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// Public CLI wire fixture only. Runner creation, detachment, persistence,
// service ownership and stop/recovery all execute their production paths.
func orcaCreationFixture(args []string) int {
	root := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	arg := func(key string) string {
		for i, a := range args {
			if a == key && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	respond := func(value any) int {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "result": value, "_meta": map[string]string{"runtimeId": os.Getenv("STEERRUNNER_CREATION_RUNTIME")}})
		return 0
	}
	if root == "" || len(args) == 0 {
		return 2
	}
	method := "orca/" + args[0]
	if len(args) > 1 {
		method += "/" + args[1]
	}
	if err := appendCall(map[string]any{"method": method, "args": args}); err != nil {
		return 2
	}
	if args[0] == "status" {
		return respond(map[string]any{"runtime": map[string]any{"state": "ready", "appVersion": "1.4.224", "capabilities": []string{"terminal.prompt-delivery.v1"}}})
	}
	type created struct {
		Worktree native.Worktree
		Terminal native.Terminal
		CLI      b.ProcessIdentity
	}
	path := filepath.Join(root, "worktree-created.json")
	var value created
	if args[0] == "worktree" && len(args) > 1 && args[1] == "create" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "fixture refused duplicate creation")
			return 2
		}
		workspace := filepath.Join(root, "original-worktree")
		if err := os.Mkdir(workspace, 0700); err != nil {
			return 2
		}
		identity, err := process.CurrentIdentity()
		if err != nil {
			return 2
		}
		value = created{Worktree: native.Worktree{ID: "creation-repo::" + workspace, RepoID: "creation-repo", InstanceID: "original-worktree-instance", Path: workspace, Branch: "original-branch", Head: arg("--base-branch"), BaseRef: arg("--base-branch"), DisplayName: arg("--name")}, CLI: *identity}
		value.Terminal = native.Terminal{Handle: "original-terminal", IncarnationID: "original-terminal-incarnation", WorktreeID: value.Worktree.ID, WorktreePath: workspace, Branch: value.Worktree.Branch, Connected: true, AgentIdentity: arg("--agent"), Status: "running"}
		raw, _ := json.Marshal(value)
		if process.New(time.Second).WriteExecutionFile(path, raw) != nil {
			return 2
		}
		for exists(filepath.Join(root, "hold-create-response")) {
			time.Sleep(20 * time.Millisecond)
		}
		if exists(filepath.Join(root, "lose-create-response")) {
			fmt.Fprintln(os.Stderr, "creation completed; response lost before any resource identity returned")
			return 1
		}
		return respond(map[string]any{"worktree": value.Worktree, "agentTerminalHandle": value.Terminal.Handle})
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &value) != nil {
		return 2
	}
	if len(args) < 2 {
		return 2
	}
	switch args[0] + " " + args[1] {
	case "worktree show":
		return respond(map[string]any{"worktree": value.Worktree})
	case "worktree list":
		// Even a perfectly matching name in an inventory is not a creation receipt.
		return respond(map[string]any{"worktrees": []any{value.Worktree}})
	case "terminal show":
		_ = process.New(time.Second).WriteExecutionFile(filepath.Join(root, "show-entered"), []byte("entered"))
		for exists(filepath.Join(root, "hold-show-response")) {
			time.Sleep(20 * time.Millisecond)
		}
		return respond(map[string]any{"terminal": value.Terminal})
	case "terminal list":
		rows := []native.Terminal{value.Terminal}
		if exists(filepath.Join(root, "original-terminal-closed")) {
			rows = []native.Terminal{}
		}
		return respond(map[string]any{"terminals": rows, "totalCount": len(rows), "truncated": false, "hostScope": map[string]any{"hostIds": []string{"creation-host"}, "omittedHostIds": []string{}}})
	case "terminal close":
		if arg("--terminal") != value.Terminal.Handle {
			return 2
		}
		_ = os.WriteFile(filepath.Join(root, "original-terminal-closed"), []byte("closed"), 0600)
		return respond(map[string]any{"close": map[string]any{"handle": value.Terminal.Handle, "ptyKilled": true}})
	}
	fmt.Fprintln(os.Stderr, "unexpected creation fixture command")
	return 2
}
