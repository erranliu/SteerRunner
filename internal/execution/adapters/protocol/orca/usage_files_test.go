package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestUsageFilesRejectOriginalIdentityBeforeDial(t *testing.T) {
	var dials, files atomic.Int32
	pairing, _ := remoteWorkspaceFileFixture(t, "usage-token", false, &dials, &files)
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	raw, err := json.Marshal(map[string]any{"environments": []any{map[string]any{
		"id": "usage-env", "preferredEndpointId": "endpoint", "endpoints": []any{map[string]string{
			"id": "endpoint", "endpoint": pairing.Endpoint, "deviceToken": pairing.Token, "publicKeyB64": pairing.PublicKey,
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	host, err := UsageHostID("usage-env")
	if err != nil {
		t.Fatal(err)
	}
	for _, opener := range []struct {
		name string
		open func(context.Context, string, string, string) (*UsageFileChannel, error)
	}{{"ordinary", OpenUsageFiles}, {"structured", OpenStructuredUsageFiles}} {
		t.Run(opener.name, func(t *testing.T) {
			for _, tc := range []struct{ environment, runtime, host, want string }{
				{"usage-env", "", host, "IDENTITY_UNVERIFIED"},
				{"usage-env", "runtime", "foreign-host", "IDENTITY_CHANGED"},
				{"usage-env", "", "foreign-host", "IDENTITY_UNVERIFIED"},
				{"missing-env", "", "foreign-host", "fixed Orca environment endpoint is unavailable"},
			} {
				channel, err := opener.open(context.Background(), tc.environment, tc.runtime, tc.host)
				if channel != nil {
					channel.Close()
				}
				if channel != nil || err == nil || err.Error() != tc.want || dials.Load() != 0 || files.Load() != 0 {
					t.Fatal("identity rejection changed or connected", tc, err, dials.Load(), files.Load())
				}
			}
		})
	}
}

func TestUsageGroupReferenceProof(t *testing.T) {
	for _, tc := range []struct {
		raw           string
		unused, valid bool
	}{
		{`{"repos":[]}`, true, true},
		{`{"repos":[{"projectGroupId":"owned"}]}`, false, true},
		{`{"repos":[{"projectGroupId":"other"}]}`, true, true},
		{`{}`, false, false},
		{`{"repos":[],"truncated":true}`, false, false},
	} {
		socket := &usageSocket{raw: json.RawMessage(tc.raw)}
		channel := UsageFileChannel{socket: socket}
		unused, err := channel.GroupUnused(context.Background(), "owned")
		if (err == nil) != tc.valid || unused != tc.unused || socket.method != "repo.list" {
			t.Fatal(tc, unused, err, socket.method)
		}
	}
}

func TestUsageIncompleteInventoriesFailClosed(t *testing.T) {
	for _, tc := range []struct {
		field string
		read  func(*UsageFileChannel) error
	}{
		{"groups", func(c *UsageFileChannel) error { _, err := c.Groups(context.Background()); return err }},
		{"folderWorkspaces", func(c *UsageFileChannel) error { _, err := c.Folders(context.Background()); return err }},
		{"repos", func(c *UsageFileChannel) error { _, err := c.GroupUnused(context.Background(), "owned"); return err }},
		{"sessions", func(c *UsageFileChannel) error { _, err := c.Sessions(context.Background()); return err }},
	} {
		for _, boundary := range []string{`"hasMore":true`, `"nextCursor":"more"`, `"totalCount":1`, `"totalCount":-1`} {
			socket := &usageSocket{raw: json.RawMessage(fmt.Sprintf(`{"%s":[],%s}`, tc.field, boundary))}
			if err := tc.read(&UsageFileChannel{socket: socket}); err == nil {
				t.Fatal("partial inventory accepted as ownership/removal proof", tc.field, boundary)
			}
		}
		socket := &usageSocket{raw: json.RawMessage(fmt.Sprintf(`{"%s":[],"totalCount":0}`, tc.field))}
		if err := tc.read(&UsageFileChannel{socket: socket}); err != nil {
			t.Fatal("complete inventory rejected", tc.field, err)
		}
	}
	socket := &usageSocket{raw: json.RawMessage(`{"groups":[],"truncated":true}`)}
	c := UsageFileChannel{socket: socket}
	if _, err := c.Groups(context.Background()); err == nil {
		t.Fatal("truncated groups allowed")
	}
	socket.raw = json.RawMessage(`{"folderWorkspaces":[],"truncated":true}`)
	if _, err := c.Folders(context.Background()); err == nil {
		t.Fatal("truncated folders allowed")
	}
	for _, raw := range []string{`{"sessions":[],"truncated":true}`, `{"sessions":[],"hasMore":true}`, `{"sessions":[],"totalCount":1}`, `{"sessions":[],"nextCursor":"more"}`} {
		socket.raw = json.RawMessage(raw)
		if _, err := c.Sessions(context.Background()); err == nil {
			t.Fatal("incomplete candidates allowed", raw)
		}
	}
}

type usageSocket struct {
	method string
	params any
	calls  int
	raw    json.RawMessage
	err    error
}

func (s *usageSocket) request(_ context.Context, method string, params any) (string, error) {
	s.method = method
	s.params = params
	s.calls++
	return "q", nil
}
func (s *usageSocket) next(context.Context, string) (json.RawMessage, error) { return s.raw, s.err }
func (s *usageSocket) close() error                                          { return nil }
func TestUsageFileRPCBoundAndRevocationProof(t *testing.T) {
	socket := &usageSocket{raw: json.RawMessage(`{"contentBase64":"YWJj","bytesRead":3,"eof":true}`)}
	channel := UsageFileChannel{socket: socket, HostID: "host"}
	bytes, err := channel.ReadChunk(context.Background(), "owned-folder", "log.jsonl", 0, 3)
	if err != nil || string(bytes) != "abc" || socket.method != "files.readChunk" {
		t.Fatal(string(bytes), err)
	}
	calls := socket.calls
	for _, file := range []string{"../auth.json", `..\auth.json`, `C:\auth.json`, "."} {
		if _, err = channel.ReadChunk(context.Background(), "f", file, 0, 3); err == nil {
			t.Fatal("path escaped", file)
		}
	}
	if _, err = channel.ReadChunk(context.Background(), "f", "log", 0, 512<<10+1); err == nil || socket.calls != calls {
		t.Fatal("read bound escaped")
	}
	for _, e := range []error{errors.New("network disconnected"), &StateRPCError{Code: "unauthorized"}, &StateRPCError{Code: "ENOENT"}, &StateRPCError{Code: "file_not_found"}} {
		socket.err = e
		if channel.SelectorRevoked(context.Background(), "owned-folder", "log.jsonl") {
			t.Fatal("invalid revocation proof", e)
		}
	}
	socket.err = &StateRPCError{Code: "selector_not_found"}
	if !channel.SelectorRevoked(context.Background(), "owned-folder", "log.jsonl") {
		t.Fatal("explicit selector proof lost")
	}
}
