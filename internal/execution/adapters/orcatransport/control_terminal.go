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

func (l *Lifecycle) usesControl() bool { return l.frozenRecord().usesControl() }

func (l *Lifecycle) controlSetup(s b.LifecycleSnapshot, state State) codex.RemoteControlSetup {
	return l.frozenRecord().controlSetup(s, state)
}

func (l *Lifecycle) validateControl(s b.LifecycleSnapshot, state State, ready bool) error {
	return l.frozenRecord().validateControl(s, state, ready)
}

// Polling sleeps stay cancellable; the fixed actor owns any in-flight query.
