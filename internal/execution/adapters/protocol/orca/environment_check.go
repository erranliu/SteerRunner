package orca

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
)

const terminalCreateID = "terminal.create-idempotency.v2"
const terminalShell = "terminal.create-shell-selection.v1"

type EnvironmentTerminalClient struct {
	Environment, Runtime, Host string
}

func (c EnvironmentTerminalClient) call(ctx context.Context, method string, params any, mutation bool, shell string) (json.RawMessage, error) {
	var conn agentSessionConnection
	var advertised []string
	capabilities := []string{terminalCreateID, terminalShell}
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

func (c EnvironmentTerminalClient) Start(ctx context.Context, worktree, id, command, shell string, env map[string]string) (Terminal, error) {
	params := map[string]any{"env": env, "worktree": "id:" + worktree, "clientMutationId": id, "command": command, "title": "SteerLine environment check", "presentation": "background"}
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
