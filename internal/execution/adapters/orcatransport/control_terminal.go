package orcatransport

import (
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const controlPurpose = "native-control"

type ControlTerminal struct {
	Process b.ProcessIdentity         `json:"process"`
	Ready   *codex.RemoteControlSetup `json:"ready,omitempty"`
}

// Polling sleeps stay cancellable; the fixed actor owns any in-flight query.
