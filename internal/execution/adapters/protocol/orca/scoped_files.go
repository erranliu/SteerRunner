package orca

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path"
	"strings"
)

type FileScope struct{ Root, RelativePath string }

func (s FileScope) valid() bool {
	valid := func(v string) bool {
		if v == "" || v == "." || v == ".." || strings.HasPrefix(v, "/") || strings.ContainsAny(v, "\\:") || path.Clean(v) != v || strings.HasPrefix(v, "../") {
			return false
		}
		for _, part := range strings.Split(v, "/") {
			if strings.TrimRight(part, " .") != part {
				return false
			}
		}
		for _, character := range v {
			if character < 32 || character == 127 {
				return false
			}
		}
		return true
	}
	return valid(s.Root) && valid(s.RelativePath) && strings.HasPrefix(s.RelativePath, s.Root+"/")
}

type FileLimits struct{ ReadBytes, WriteBytes int }

func (l FileLimits) valid() bool {
	return l.ReadBytes > 0 && l.ReadBytes < int(^uint(0)>>1)/4-2 && l.WriteBytes > 0
}

type WorkspaceFileBinding struct {
	Environment, RuntimeID, ExecutionHostID, WorktreeID, WorkspacePath, InstanceID string
}

// ScopedFiles owns a connection to one explicit workspace file. Paths and
// finite byte budgets are supplied by its caller, without interpreting a Run.
type ScopedFiles struct {
	socket             stateConnection
	selector, relative string
	limits             FileLimits
	Instance           string
}

func OpenWorkspaceFiles(ctx context.Context, binding WorkspaceFileBinding, scope FileScope, limits FileLimits) (*ScopedFiles, error) {
	return openWorkspaceFiles(ctx, binding, scope, limits, resolveStatePairing)
}

func openWorkspaceFiles(ctx context.Context, binding WorkspaceFileBinding, scope FileScope, limits FileLimits, resolve func(string) (statePairing, error)) (*ScopedFiles, error) {
	environment, runtime, host, worktree, workspace, instance := binding.Environment, binding.RuntimeID, binding.ExecutionHostID, binding.WorktreeID, binding.WorkspacePath, binding.InstanceID
	if environment == "" && host != "local" || runtime == "" || worktree == "" || workspace == "" || !scope.valid() || !limits.valid() {
		return nil, errors.New("binding_invalid")
	}
	var socket stateConnection
	var err error
	if environment == "" {
		socket, err = openStateConnection(ctx, environment, runtime)
	} else {
		pairing, resolveErr := resolve(environment)
		if resolveErr != nil {
			return nil, errors.New("binding_invalid")
		}
		pairedHost, keyErr := pairing.hostID()
		if keyErr != nil || pairedHost != host {
			return nil, errors.New("binding_invalid")
		}
		// Dial with the exact pairing whose public key was just checked.
		socket, err = openStateSocket(ctx, pairing, runtime)
	}
	if err != nil {
		return nil, errors.New("transport_unavailable")
	}
	fail := func() (*ScopedFiles, error) { socket.close(); return nil, errors.New("binding_invalid") }
	raw, err := callState(ctx, socket, "worktree.show", map[string]any{"worktree": "id:" + worktree})
	if err != nil {
		socket.close()
		return nil, errors.New("transport_unavailable")
	}
	var value struct {
		Worktree Worktree `json:"worktree"`
	}
	if json.Unmarshal(raw, &value) != nil || value.Worktree.ID != worktree || !strings.EqualFold(strings.ReplaceAll(value.Worktree.Path, "\\", "/"), strings.ReplaceAll(workspace, "\\", "/")) || instance != "" && value.Worktree.InstanceID != instance {
		return fail()
	}
	return &ScopedFiles{socket: socket, selector: "id:" + worktree, relative: scope.RelativePath, limits: limits, Instance: value.Worktree.InstanceID}, nil
}
func (c *ScopedFiles) Close() error { return c.socket.close() }
func (c *ScopedFiles) Initialize(ctx context.Context, raw []byte) error {
	if len(raw) > c.limits.WriteBytes {
		return errors.New("size_exceeded")
	}
	_, err := callState(ctx, c.socket, "files.writeBase64", map[string]any{"worktree": c.selector, "relativePath": c.relative, "contentBase64": base64.StdEncoding.EncodeToString(raw), "expectedExecutionHostId": "local"})
	if err != nil {
		return errors.New("initialization_unverified")
	}
	return nil
}
func (c *ScopedFiles) Read(ctx context.Context) ([]byte, error) {
	raw, err := callState(ctx, c.socket, "files.readChunk", map[string]any{"worktree": c.selector, "relativePath": c.relative, "offset": 0, "length": c.limits.ReadBytes + 1})
	if err != nil {
		return nil, errors.New("read_failed")
	}
	var value struct {
		Content string `json:"contentBase64"`
		Bytes   *int   `json:"bytesRead"`
		EOF     *bool  `json:"eof"`
	}
	if json.Unmarshal(raw, &value) != nil || value.Bytes == nil || value.EOF == nil || *value.Bytes < 0 || len(value.Content) > ((c.limits.ReadBytes+2)/3)*4+4 {
		return nil, errors.New("read_failed")
	}
	if *value.Bytes > c.limits.ReadBytes || !*value.EOF {
		return nil, errors.New("size_exceeded")
	}
	body, err := base64.StdEncoding.Strict().DecodeString(value.Content)
	if err != nil || len(body) != *value.Bytes {
		return nil, errors.New("read_failed")
	}
	return body, nil
}
