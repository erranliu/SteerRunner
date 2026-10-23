package orca

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const ProgressLimit = 65536

// ProgressFiles is a finite original-host channel, never a terminal reader.
// Its scope is supplied by Runner's frozen binding, not by public query input.
type ProgressFiles struct {
	socket             stateConnection
	selector, relative string
	Instance           string
}

func OpenProgressFiles(ctx context.Context, environment, runtime, host, worktree, workspace, instance, run string) (*ProgressFiles, error) {
	if environment == "" && host != "local" {
		return nil, errors.New("binding_invalid")
	}
	if runtime == "" || worktree == "" || workspace == "" || len(run) != 32 || strings.ContainsAny(run, "/\\.:\x00\r\n") {
		return nil, errors.New("binding_invalid")
	}
	if environment != "" {
		actual, err := UsageHostID(environment)
		if err != nil || actual != host {
			return nil, errors.New("binding_invalid")
		}
	}
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return nil, errors.New("transport_unavailable")
	}
	fail := func() (*ProgressFiles, error) { socket.close(); return nil, errors.New("binding_invalid") }
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
	return &ProgressFiles{socket: socket, selector: "id:" + worktree, relative: ".steerrunner/progress/" + run + "/progress.bin", Instance: value.Worktree.InstanceID}, nil
}
func (c *ProgressFiles) Close() error { return c.socket.close() }
func (c *ProgressFiles) Initialize(ctx context.Context, raw []byte) error {
	if len(raw) > ProgressLimit {
		return errors.New("size_exceeded")
	}
	_, err := callState(ctx, c.socket, "files.writeBase64", map[string]any{"worktree": c.selector, "relativePath": c.relative, "contentBase64": base64.StdEncoding.EncodeToString(raw), "expectedExecutionHostId": "local"})
	if err != nil {
		return errors.New("initialization_unverified")
	}
	return nil
}
func (c *ProgressFiles) Read(ctx context.Context) ([]byte, error) {
	raw, err := callState(ctx, c.socket, "files.readChunk", map[string]any{"worktree": c.selector, "relativePath": c.relative, "offset": 0, "length": ProgressLimit + 1})
	if err != nil {
		return nil, errors.New("read_failed")
	}
	var value struct {
		Content string `json:"contentBase64"`
		Bytes   *int   `json:"bytesRead"`
		EOF     *bool  `json:"eof"`
	}
	if json.Unmarshal(raw, &value) != nil || value.Bytes == nil || value.EOF == nil || *value.Bytes < 0 || len(value.Content) > 87388 {
		return nil, errors.New("read_failed")
	}
	if *value.Bytes > ProgressLimit || !*value.EOF {
		return nil, errors.New("size_exceeded")
	}
	body, err := base64.StdEncoding.Strict().DecodeString(value.Content)
	if err != nil || len(body) != *value.Bytes {
		return nil, errors.New("read_failed")
	}
	return body, nil
}
