package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

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
