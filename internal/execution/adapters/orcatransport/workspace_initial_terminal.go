package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type initialTerminalCheckpoint struct {
	Version   int                `json:"version"`
	Binding   string             `json:"binding"`
	Attempted bool               `json:"attempted"`
	Process   *b.ProcessIdentity `json:"process,omitempty"`
}

// Only a confirmed newly-created workspace with a complete empty terminal
// inventory can acquire its first terminal. Persist its single creation intent
// before the effect. Unknown creation never permits a replacement.
func (l *Lifecycle) initialWorkspaceTerminal(ctx context.Context, s b.LifecycleSnapshot, c workspaceCreation) (native.Terminal, error) {
	if !filepath.IsAbs(l.setup.RunDirectory) {
		return native.Terminal{}, errors.New("caller initial-terminal checkpoint directory missing")
	}
	raw, _ := json.Marshal(struct {
		Snapshot b.LifecycleSnapshot
		Runtime  string
		Worktree native.Worktree
	}{Snapshot: b.LifecycleSnapshot{Session: s.Session, ResourceGeneration: s.ResourceGeneration, ConfigurationDigest: s.ConfigurationDigest}, Runtime: c.RuntimeID, Worktree: c.Worktree})
	binding := digest(raw)
	path := filepath.Join(l.setup.RunDirectory, "orca-initial-terminals", binding+".json")
	confirm := func(p b.ProcessIdentity) (native.Terminal, error) {
		t, e := l.soleWorkspaceTerminal(ctx, c.Worktree.ID, c.RuntimeID)
		if e != nil || t.Handle != p.Terminal || t.IncarnationID != p.Incarnation {
			return t, errors.Join(e, errors.New("original initial terminal inventory changed"))
		}
		return t, nil
	}
	saved := initialTerminalCheckpoint{Version: 1, Binding: binding}
	data, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return native.Terminal{}, e
	}
	if e == nil && (len(data) > 1<<20 || b.DecodeStrictJSON(data, &saved, "trailing initial terminal checkpoint") != nil || saved.Version != 1 || saved.Binding != binding || !saved.Attempted) {
		return native.Terminal{}, errors.New("initial terminal checkpoint invalid or foreign")
	}
	if saved.Attempted {
		if saved.Process == nil {
			return native.Terminal{}, errors.New("original initial terminal creation outcome unknown; no replacement created")
		}
		p := *saved.Process
		if p.Kind != "orca_terminal" || p.RuntimeID != c.RuntimeID || p.WorktreeID != c.Worktree.ID || p.Terminal == "" || p.Incarnation == "" {
			return native.Terminal{}, errors.New("initial terminal checkpoint identity changed")
		}
		t, e := l.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, p)
		if e != nil || t.WorktreeID != c.Worktree.ID {
			return t, errors.Join(e, errors.New("original initial terminal unavailable"))
		}
		return confirm(p)
	}
	emptySince := time.Now()
	for {
		r, e := l.workspaceTerminalQuery(ctx, "terminal", "list", "--worktree", "id:"+c.Worktree.ID, "--json")
		if e != nil {
			return native.Terminal{}, e
		}
		var v struct {
			Terminals []native.Terminal
			Truncated *bool
			Total     *int `json:"totalCount"`
			Hosts     *struct {
				IDs     []string `json:"hostIds"`
				Omitted []string `json:"omittedHostIds"`
			} `json:"hostScope"`
		}
		if json.Unmarshal(r.Payload, &v) != nil || r.RuntimeID != c.RuntimeID || v.Truncated != nil && *v.Truncated {
			return native.Terminal{}, errors.New("original initial terminal inventory unverified")
		}
		if len(v.Terminals) > 1 {
			return native.Terminal{}, errors.New("workspace has more than one terminal; no startup command was sent")
		}
		if len(v.Terminals) == 1 {
			t := v.Terminals[0]
			if t.Handle == "" || t.IncarnationID == "" || t.WorktreeID != c.Worktree.ID || t.Orphaned {
				return t, errors.New("original initial terminal identity changed")
			}
			return t, nil
		}
		complete := v.Truncated != nil && !*v.Truncated && v.Total != nil && *v.Total == 0 && v.Hosts != nil && len(v.Hosts.IDs) > 0 && len(v.Hosts.Omitted) == 0
		if complete && time.Since(emptySince) >= 2*time.Second {
			saved.Attempted = true
			write := func() error { raw, _ := json.Marshal(saved); return l.setup.ProcessHost.WriteExecutionFile(path, raw) }
			if e = write(); e != nil {
				return native.Terminal{}, e
			}
			r, e = l.client.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+c.Worktree.ID, "--title", "SteerRunner initial execution "+c.Worktree.DisplayName, "--json")
			var made struct{ Terminal native.Terminal }
			if e != nil || r.CallUncertain || r.RuntimeID != c.RuntimeID || json.Unmarshal(r.Payload, &made) != nil || made.Terminal.Handle == "" || made.Terminal.IncarnationID == "" || made.Terminal.WorktreeID != c.Worktree.ID {
				return native.Terminal{}, errors.New("original initial terminal creation unverified; no replacement created")
			}
			p := b.ProcessIdentity{Kind: "orca_terminal", Executable: l.profile.Executable, RuntimeID: r.RuntimeID, Terminal: made.Terminal.Handle, Incarnation: made.Terminal.IncarnationID, WorktreeID: c.Worktree.ID, ExecutionHost: l.profile.Target().ExecutionHost}
			saved.Process = &p
			if e = write(); e != nil {
				return native.Terminal{}, e
			}
			return confirm(p)
		}
		if e = waitWorkspaceTerminal(ctx); e != nil {
			return native.Terminal{}, e
		}
	}
}
