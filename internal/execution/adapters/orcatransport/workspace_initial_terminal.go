package orcatransport

import (
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type initialTerminalCheckpoint struct {
	Version         int                `json:"version"`
	Binding         string             `json:"binding"`
	WorktreeBinding string             `json:"worktree_binding,omitempty"`
	Attempted       bool               `json:"attempted"`
	Process         *b.ProcessIdentity `json:"process,omitempty"`
}

// This plan precedes workspace creation, so caller loss while that original
// creation owner is running does not turn an unstarted terminal phase into an
// unknown terminal creation. No legacy journal is inferred or migrated.

// Only a confirmed newly-created workspace with a complete empty terminal
// inventory can acquire its first terminal. Persist its single creation intent
// before the effect. Unknown creation never permits a replacement.
