package application

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"testing"
)

type viewerPort struct {
	b.LifecyclePort // Any execution action unexpectedly invoked fails the test.
	reads           int
	err             error
}

func (p *viewerPort) NativeViewerCommand(context.Context, b.LifecycleSnapshot) (b.LaunchCommand, error) {
	p.reads++
	return b.LaunchCommand{Executable: "fixed.exe", Args: []string{"resume", "--remote", "ws://127.0.0.1:12345", "session"}}, p.err
}

func TestNativeViewerUsesOriginalTargetWithoutExecution(t *testing.T) {
	for _, tc := range []string{"ready", "attempt", "generation", "session", "missing", "unsupported", "unverified", "stopping", "launch_failed"} {
		t.Run(tc, func(t *testing.T) {
			r := &runstate.Record{ID: "run", Revision: 8, Status: "running", Allowed: true}
			r.Request.Execution.Adapter = "codex-app-server"
			r.Snapshot.Session = b.SessionContext{AttemptID: "attempt", SessionID: "session"}
			r.Snapshot.ResourceGeneration = "generation"
			target := contracts.NativeSessionTarget{RunID: "run", AttemptID: "attempt", SessionID: "session", Generation: "generation"}
			p := &viewerPort{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e := &Engine{ctx: ctx, runs: map[string]*runstate.Record{"run": r}, workers: map[string]*worker{"run": {port: p}}}
			opened := 0
			e.openNativeConsole = func(context.Context, b.LaunchCommand) error {
				opened++
				if tc == "launch_failed" {
					return errors.New("launch failed")
				}
				return nil
			}
			switch tc {
			case "attempt":
				target.AttemptID = "old"
			case "generation":
				target.Generation = "old"
			case "session":
				target.SessionID = "other"
			case "missing":
				target.RunID = "missing"
			case "unsupported":
				r.Request.Execution.Adapter = "orca-terminal"
			case "unverified":
				p.err = errors.New("original native resource missing")
			case "stopping":
				cancel()
			}
			before := r.Clone()
			response := e.OpenNativeSession(target)
			if response.OK != (tc == "ready") || !reflect.DeepEqual(before, r.Clone()) {
				t.Fatal("viewer changed execution or accepted wrong target", response)
			}
			wantOpen, wantRead := 0, 0
			if tc == "ready" || tc == "launch_failed" {
				wantOpen = 1
			}
			if wantOpen == 1 || tc == "unverified" {
				wantRead = 1
			}
			if opened != wantOpen || p.reads != wantRead {
				t.Fatal("stale target invoked native viewer", opened, p.reads)
			}
		})
	}
}
