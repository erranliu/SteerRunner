package orca

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
)

const terminalCreateID = "terminal.create-idempotency.v2"
const terminalShell = "terminal.create-shell-selection.v1"

type EnvironmentTerminalClient struct {
	Environment, Runtime, Host string
}

func (c EnvironmentTerminalClient) call(ctx context.Context, method string, params any, mutation bool, shell string) (json.RawMessage, error) {
	var conn agentSessionConnection
	var advertised []string
	capabilities := append(remoteAgentSessionCapabilities(), terminalCreateID, terminalShell)
	if c.Environment == "" {
		if c.Runtime == "" {
			var err error
			c.Runtime, err = LocalRuntimeID()
			if err != nil {
				return nil, err
			}
		}
		local, err := openLocalState(ctx, c.Runtime)
		if err != nil {
			return nil, err
		}
		conn, advertised = local, local.capabilities
	} else {
		pairing, err := resolveStatePairing(c.Environment)
		if err != nil {
			return nil, err
		}
		host, err := pairing.hostID()
		if err != nil || host != c.Host {
			return nil, stateIdentityError("original environment host changed")
		}
		remote, err := openStateSocketMode(ctx, pairing, c.Runtime, capabilities, c.Runtime == "")
		if err != nil {
			return nil, err
		}
		conn, advertised = remote, remote.capabilities
	}
	defer conn.close()
	if method == "terminal.create" && (!slices.Contains(advertised, terminalCreateID) || shell != "" && !slices.Contains(advertised, terminalShell)) {
		return nil, errors.New("Orca host lacks the required terminal creation capabilities")
	}
	id, err := conn.requestWithCapabilities(ctx, method, params, capabilities, mutation)
	if err != nil {
		return nil, err
	}
	return conn.next(ctx, id)
}

// ExpectedHandle is bound to the original runtime and authenticated caller.
// Querying it after loss of the create reply does not reissue the mutation.
func (c EnvironmentTerminalClient) ExpectedHandle(ctx context.Context, worktree, operation string) (string, error) {
	caller := "local"
	if c.Environment != "" {
		raw, err := c.call(ctx, "status.get", nil, false, "")
		if err != nil {
			return "", err
		}
		var status struct{ PairedDeviceID string }
		if json.Unmarshal(raw, &status) != nil || status.PairedDeviceID == "" {
			return "", errors.New("Orca paired caller identity is unavailable")
		}
		caller = status.PairedDeviceID
	}
	hash := sha256.Sum256([]byte("orca.remote-terminal-create.v2\x00" + caller + "\x00" + worktree + "\x00" + operation))
	return fmt.Sprintf("term_%x", hash[:16]), nil
}

// CheckoutInventory discovers only the registered repositories on the paired host.
func (c EnvironmentTerminalClient) CheckoutInventory(ctx context.Context) (RepositoryInventory, string, error) {
	var runtime string
	if c.Environment == "" {
		var err error
		runtime, err = LocalRuntimeID()
		if err != nil {
			return RepositoryInventory{}, "", err
		}
		c.Runtime = runtime
	} else {
		pairing, err := resolveStatePairing(c.Environment)
		if err != nil {
			return RepositoryInventory{}, "", err
		}
		host, err := pairing.hostID()
		if err != nil || host != c.Host {
			return RepositoryInventory{}, "", stateIdentityError("original environment host changed")
		}
		conn, err := openStateSocketMode(ctx, pairing, "", nil, true)
		if err != nil {
			return RepositoryInventory{}, "", err
		}
		defer conn.close()
		raw, err := callState(ctx, conn, "repo.list", nil)
		return DecodeRepositoryInventory(Result{Payload: raw}), conn.runtime, err
	}
	raw, err := c.call(ctx, "repo.list", nil, false, "")
	return DecodeRepositoryInventory(Result{Payload: raw}), runtime, err
}

func (c EnvironmentTerminalClient) Checkout(ctx context.Context, path string) (Worktree, error) {
	raw, err := c.call(ctx, "worktree.show", map[string]string{"worktree": "path:" + path}, false, "")
	var query WorktreeQuery
	if err == nil {
		err = json.Unmarshal(raw, &query)
	}
	return query.Worktree, err
}

func (c EnvironmentTerminalClient) Start(ctx context.Context, worktree, id, title, command, shell string, env map[string]string) (Terminal, error) {
	params := map[string]any{"env": env, "worktree": "id:" + worktree, "clientMutationId": id, "command": command, "title": title, "presentation": "background"}
	if shell != "" {
		params["shell"] = shell
	}
	raw, err := c.call(ctx, "terminal.create", params, true, shell)
	var value struct{ Terminal Terminal }
	if err == nil {
		err = json.Unmarshal(raw, &value)
	}
	value.Terminal.RuntimeID = c.Runtime
	return value.Terminal, err
}

func (c EnvironmentTerminalClient) Terminal(ctx context.Context, id TerminalIdentity) (Terminal, error) {
	raw, err := c.call(ctx, "terminal.show", map[string]string{"terminal": id.Handle}, false, "")
	var value struct{ Terminal Terminal }
	if err == nil {
		err = json.Unmarshal(raw, &value)
	}
	if err != nil {
		return Terminal{}, err
	}
	value.Terminal.RuntimeID = c.Runtime
	if id.IncarnationID == "" && value.Terminal.Handle == id.Handle && value.Terminal.IncarnationID != "" {
		return value.Terminal, nil
	}
	return value.Terminal, validateTerminalIdentity(value.Terminal, id)
}

func (c EnvironmentTerminalClient) Exit(ctx context.Context, handle string) (*int, error) {
	raw, err := c.call(ctx, "terminal.wait", map[string]any{"terminal": handle, "for": "exit", "timeoutMs": 750}, false, "")
	if err != nil {
		var rpc *StateRPCError
		if errors.As(err, &rpc) && (rpc.Code == "timeout" || rpc.Message == "timeout") {
			return nil, nil
		}
		return nil, err
	}
	var value struct {
		Wait struct {
			Handle, Condition, Status string
			Satisfied                 bool
			ExitCode                  *int
		}
	}
	if err = json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	w := value.Wait
	if w.Handle != handle || w.Condition != "exit" {
		return nil, errors.New("terminal exit receipt identity differs")
	}
	if !w.Satisfied {
		return nil, nil
	}
	if w.Status != "exited" || w.ExitCode == nil {
		return nil, errors.New("original terminal exit code is unavailable")
	}
	return w.ExitCode, nil
}

func (c EnvironmentTerminalClient) Read(ctx context.Context, handle string) ([]string, error) {
	var output []string
	cursor := 0
	for len(output) < 512 {
		raw, err := c.call(ctx, "terminal.read", map[string]any{"terminal": handle, "cursor": cursor, "limit": 128}, false, "")
		if err != nil {
			return nil, err
		}
		var value struct {
			Terminal struct {
				Handle                   string
				Lines                    []string `json:"tail"`
				NextCursor, OldestCursor string
				Limited, Truncated       bool
			}
		}
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		t := value.Terminal
		oldest, oldestErr := strconv.Atoi(t.OldestCursor)
		next, nextErr := strconv.Atoi(t.NextCursor)
		if t.Handle != handle || oldestErr != nil || nextErr != nil || oldest < 0 || oldest > cursor || next < cursor || t.Truncated {
			return nil, errors.New("original environment terminal output was truncated")
		}
		output = append(output, t.Lines...)
		if !t.Limited {
			return output, nil
		}
		if next <= cursor {
			return nil, errors.New("original terminal output cursor did not advance")
		}
		cursor = next
	}
	return nil, errors.New("original environment terminal output exceeds bounds")
}

func (c EnvironmentTerminalClient) Stop(ctx context.Context, id TerminalIdentity) error {
	raw, err := c.call(ctx, "terminal.close", map[string]string{"terminal": id.Handle, "expectedIncarnationId": id.IncarnationID}, true, "")
	if err != nil {
		return err
	}
	var value struct{ Close TerminalClosure }
	if json.Unmarshal(raw, &value) != nil || value.Close.Handle != id.Handle || !value.Close.PtyKilled {
		return errors.New("original environment terminal stop is unverified")
	}
	return nil
}

// CheckoutIdle conservatively observes original-host checkout users.
func (c EnvironmentTerminalClient) CheckoutIdle(ctx context.Context, worktree string) error {
	selector := "id:" + worktree
	raw, err := c.call(ctx, "terminal.list", map[string]any{"worktree": selector, "requireFreshPtyLiveness": true, "includeVisualLayouts": false}, false, "")
	if err != nil {
		return err
	}
	inventory := DecodeTerminalInventory(Result{Payload: raw})
	if !inventory.Decoded() || inventory.Terminals == nil || inventory.Total == nil || *inventory.Total != len(inventory.Terminals) ||
		inventory.Truncated == nil || *inventory.Truncated || inventory.Hosts == nil || inventory.Hosts.Omitted == nil || !slices.Contains(inventory.Hosts.IDs, "local") ||
		slices.Contains(inventory.Hosts.Omitted, "local") {
		return errors.New("Existing checkout terminal inventory is incomplete.")
	}
	inspected := make(map[string]string, len(inventory.Terminals))
	for _, terminal := range inventory.Terminals {
		if terminal.Handle == "" || terminal.IncarnationID == "" || terminal.WorktreeID != worktree || inspected[terminal.Handle] != "" {
			return errors.New("Existing checkout terminal inventory identity changed.")
		}
		if err := c.checkCheckoutTerminalIdle(ctx, terminal); err != nil {
			return err
		}
		inspected[terminal.Handle] = terminal.IncarnationID
	}
	raw, err = c.call(ctx, "session.tabs.list", map[string]string{"worktree": selector}, false, "")
	if err != nil {
		return err
	}
	var sessions struct {
		Worktree string
		Tabs     []struct {
			Terminal      string
			IncarnationID string
		}
		Truncated bool
	}
	if json.Unmarshal(raw, &sessions) != nil || sessions.Worktree != worktree || sessions.Tabs == nil || sessions.Truncated {
		return errors.New("Existing checkout session inventory is incomplete.")
	}
	for _, tab := range sessions.Tabs {
		if tab.Terminal == "" || tab.IncarnationID == "" {
			return errors.New("Existing checkout is occupied by an Orca session tab.")
		}
		if incarnation := inspected[tab.Terminal]; incarnation != "" {
			if incarnation != tab.IncarnationID {
				return errors.New("Existing checkout session tab terminal identity changed.")
			}
			continue
		}
		terminal, err := c.Terminal(ctx, TerminalIdentity{RuntimeID: c.Runtime, Handle: tab.Terminal, IncarnationID: tab.IncarnationID})
		if err != nil || terminal.WorktreeID != worktree {
			return errors.New("Existing checkout session tab identity could not be verified.")
		}
		if !terminal.Exited() {
			code, err := c.Exit(ctx, terminal.Handle)
			if err != nil || code == nil {
				return errors.New("Existing checkout session tab is active or its exit could not be verified.")
			}
		}
	}
	return nil
}

// A live PTY can be an idle shell. Require the host's child-process evidence
// as well as a shell in the foreground; older hosts' boolean alone is ambiguous.
func (c EnvironmentTerminalClient) checkCheckoutTerminalIdle(ctx context.Context, terminal Terminal) error {
	raw, err := c.call(ctx, "terminal.inspectProcess", map[string]any{
		"terminal": terminal.Handle, "expectedIncarnationId": terminal.IncarnationID, "scanChildProcesses": true,
	}, false, "")
	if err != nil {
		return fmt.Errorf("Existing checkout terminal activity could not be verified: %w", err)
	}
	var result struct {
		Process struct {
			ForegroundProcess    string
			HasChildProcesses    *bool
			ChildProcessEvidence string
		}
	}
	if json.Unmarshal(raw, &result) != nil || result.Process.HasChildProcesses == nil {
		return errors.New("Existing checkout terminal activity could not be verified.")
	}
	p := result.Process
	if p.ChildProcessEvidence == "children" {
		return errors.New("Existing checkout is occupied by a running Orca terminal command.")
	}
	if p.ChildProcessEvidence != "no-children" || *p.HasChildProcesses || p.ForegroundProcess == "" {
		return errors.New("Existing checkout terminal activity could not be verified.")
	}
	name := strings.ToLower(path.Base(strings.ReplaceAll(p.ForegroundProcess, "\\", "/")))
	shell := strings.TrimSuffix(name, ".exe")
	switch shell {
	case "cmd", "powershell", "pwsh":
		return c.checkCheckoutWindowsPrompt(ctx, terminal, shell)
	case "bash", "dash", "fish", "ksh", "sh", "zsh":
		if windowsDrivePath(terminal.WorktreePath) {
			return errors.New("Existing checkout terminal activity could not be verified.")
		}
		return nil
	default:
		return errors.New("Existing checkout is occupied by a running Orca terminal command.")
	}
}

// Orca on Windows can report no-children while a shell command is running.
// Require the current screen to corroborate the shell's idle prompt.
func (c EnvironmentTerminalClient) checkCheckoutWindowsPrompt(ctx context.Context, terminal Terminal, shell string) error {
	raw, err := c.call(ctx, "terminal.read", map[string]any{
		"terminal": terminal.Handle, "expectedIncarnationId": terminal.IncarnationID, "screen": true, "limit": 1,
	}, false, "")
	if err != nil {
		return fmt.Errorf("Existing checkout terminal prompt could not be verified: %w", err)
	}
	var result struct {
		Terminal struct {
			Handle, Status, Source string
			Tail                   []string
			TailRows               []struct {
				Wrapped        *bool
				TrailingSpaces int
			}
		}
	}
	if json.Unmarshal(raw, &result) != nil {
		return errors.New("Existing checkout terminal prompt could not be verified.")
	}
	screen := result.Terminal
	if screen.Handle != terminal.Handle || screen.Status != "running" || screen.Source != "screen" ||
		len(screen.Tail) == 0 || len(screen.TailRows) != len(screen.Tail) {
		return errors.New("Existing checkout terminal prompt could not be verified.")
	}
	prompt, row := screen.Tail[len(screen.Tail)-1], screen.TailRows[len(screen.TailRows)-1]
	if row.Wrapped == nil || *row.Wrapped {
		return errors.New("Existing checkout terminal prompt could not be verified.")
	}
	if shell == "powershell" || shell == "pwsh" {
		if !strings.HasPrefix(prompt, "PS ") || row.TrailingSpaces != 1 {
			return errors.New("Existing checkout Orca terminal is not at an idle shell prompt.")
		}
		prompt = strings.TrimPrefix(prompt, "PS ")
	}
	directory, ok := strings.CutSuffix(prompt, ">")
	if !ok || !windowsDrivePath(directory) || strings.ContainsAny(directory[3:], "<>|:?*") {
		return errors.New("Existing checkout Orca terminal is not at an idle shell prompt.")
	}
	return nil
}

func windowsDrivePath(value string) bool {
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}
