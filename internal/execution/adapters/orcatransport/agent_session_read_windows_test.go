package orcatransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// A queued next command is legal while the same development resource still
// stores the preceding turn. File observation must retain that distinction.
func TestStructuredFilesWhileNextInputQueued(t *testing.T) {
	for _, scenario := range []string{"development-queued", "execute-queued", "changed-input", "foreign-resource", "wrong-instance", "malformed-state"} {
		t.Run(scenario, func(t *testing.T) {
			l, s, _ := structuredFixture(t)
			state, err := l.agentState(s)
			if err != nil {
				t.Fatal(err)
			}
			s.Workspace.Path = t.TempDir()
			s.Ref.WorkspacePath = s.Workspace.Path
			s.Ref.External.RuntimeID = "runtime"
			state.RuntimeID, state.Worktree.Path = "runtime", s.Workspace.Path
			setup := l.setup.Clone()
			development := scenario != "execute-queued"
			version := AgentSessionCloseProfileVersion
			if !development {
				version = AgentSessionVersion
			}
			setup = fixtureProfile(t, setup, AgentSessionProfileKey, version, AgentSessionProfile{Transport: l.profile, AccessMode: "full_access", Development: development})
			s.PendingTurn = &b.TurnInput{CommandID: strings.Repeat("d", 32), Prompt: "next original input", Purpose: "message"}
			s.Session.PendingCommandID = s.PendingTurn.CommandID
			if scenario == "changed-input" {
				s.PendingTurn.CommandID = state.CommandID
				s.Session.PendingCommandID = state.CommandID
			}
			if scenario == "foreign-resource" {
				s.Ref.External.WorktreeID = "replacement"
			}
			s.Implementation, err = l.agentEnvelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "malformed-state" {
				s.Implementation[0].Payload = json.RawMessage(`!`)
			}
			setup.Snapshot = s
			setup = setup.Clone()
			before := setup.Clone()
			port, err := NewLifecycle(setup)
			if err != nil {
				t.Fatal("current constructor rejected fixture", err)
			}
			_, stateErr := port.(*Lifecycle).agentState(setup.Snapshot)
			if (stateErr == nil) != (scenario == "development-queued" || scenario == "wrong-instance") {
				t.Fatal("current state reader disagrees with original binding", stateErr)
			}
			var calls, reads atomic.Int32
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				calls.Add(1)
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "worktree.show":
					ipcReply(conn, req, "runtime", map[string]any{"worktree": state.Worktree})
				case "files.readChunk":
					var params map[string]any
					if err := json.Unmarshal(req.Params, &params); err != nil {
						t.Error(err)
						return
					}
					progress := ".steerrunner/progress/" + s.Ref.RunID + "/progress.bin"
					result := ".steerrunner/result/" + s.Ref.RunID + "/result.bin"
					if params["worktree"] != "id:"+state.Worktree.ID || params["relativePath"] != progress && params["relativePath"] != result {
						t.Error("file query lost original scope", params)
						return
					}
					reads.Add(1)
					ipcReply(conn, req, "runtime", map[string]any{"contentBase64": "AP8=", "bytesRead": 2, "eof": true})
				default:
					t.Error("file observation dispatched an unexpected operation", req.Method)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			instance := state.Worktree.InstanceID
			if scenario == "wrong-instance" {
				instance = "replacement"
			}
			for _, open := range []func(context.Context, b.LifecycleSetup, string, string) (*native.ScopedFiles, error){ProgressSource, ResultSource} {
				source, err := open(ctx, setup, "local", instance)
				if scenario != "development-queued" {
					if source != nil || err == nil || err.Error() != "binding_invalid" || calls.Load() != 0 {
						t.Fatal("invalid file binding reached RPC", source, err, calls.Load())
					}
					continue
				}
				if err != nil {
					t.Fatal("legal next-input window rejected", err)
				}
				data, err := source.Read(ctx)
				_ = source.Close()
				if err != nil || !bytes.Equal(data, []byte{0, 255}) {
					t.Fatal(data, err)
				}
			}
			if scenario == "development-queued" && reads.Load() != 2 {
				t.Fatal("both original file channels must read", reads.Load())
			}
			if !reflect.DeepEqual(setup, before) {
				t.Fatal("file observation changed frozen input")
			}
		})
	}
}
