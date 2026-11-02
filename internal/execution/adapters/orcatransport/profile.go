package orcatransport

import (
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"time"
)

const ProfileKey = "orca.terminal-profile"
const ProfileVersion = 6
const NativeRemoteProfileVersion = 4
const FullAccessTerminalProfileVersion = 6
const NativeProfileVersion = 9
const NativeStateVersion = 9
const LocalStateVersion = 8
const StateKey = "orca.terminal-state"
const StateVersion = 7

// Environment is routing only; local and paired execution have the same ports.
type Profile struct {
	NativeTerminal             bool            `json:"native_terminal,omitempty"`
	Terminal                   *TerminalConfig `json:"terminal,omitempty"`
	RuntimeSHA256              string          `json:"runtime_sha256,omitempty"`
	RuntimeExecutable          string          `json:"runtime_executable,omitempty"`
	RuntimeConfigurationDigest string          `json:"runtime_configuration_digest,omitempty"`
	Executable                 string          `json:"executable"`
	Args                       []string        `json:"args,omitempty"`
	Environment                string          `json:"environment,omitempty"`
	WaitTimeout                string          `json:"wait_timeout"`
}

func (p Profile) nativeTerminalContract() bool { return p.NativeTerminal || p.Environment != "" }

func (p Profile) stateCodec() int {
	if p.NativeTerminal {
		return NativeStateVersion
	}
	if p.Environment == "" {
		return LocalStateVersion
	}
	return StateVersion
}

func DecodeProfile(data []byte) (Profile, error) {
	var p Profile
	if err := b.DecodeStrictJSON(data, &p, "trailing Orca terminal profile"); err != nil {
		return p, err
	}
	if strings.TrimSpace(p.Executable) == "" || strings.ContainsAny(p.Environment, "\x00\r\n") {
		return p, errors.New("Orca executable is required and environment must be a single line")
	}
	if d, err := time.ParseDuration(p.WaitTimeout); err != nil || d <= 0 || d > time.Minute {
		return p, errors.New("Orca wait_timeout must be positive and at most one minute")
	}
	for _, arg := range p.Args {
		if strings.HasPrefix(arg, "--environment") || strings.HasPrefix(arg, "--pairing-code") {
			return p, errors.New("Orca routing belongs to the environment field")
		}
	}
	return p, nil
}

func (p Profile) Target() b.ExecutionTarget {
	wire, _ := json.Marshal(struct {
		Executable  string
		Args        []string
		Environment string
	}{p.Executable, append([]string(nil), p.Args...), p.Environment})
	host := p.Environment
	if host == "" {
		host = "local"
	}
	return b.ExecutionTarget{ID: "orca-terminal:" + digest(wire)[:24], ExternalRuntime: p.Environment, ExecutionHost: host}
}
