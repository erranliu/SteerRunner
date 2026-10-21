// Package host owns the resident service and its local transport.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

const TimeoutSeconds = 30

// JSON line separators can expand when marshalled; this is a transport bound,
// while the CLI enforces the public request's original 8 MiB bound.
const maxWireBytes = 2*contracts.MaxRequestBytes + 1024*1024

type Options struct {
	ExpectedBinding    *contracts.ExpectedServiceBinding `json:"expected_binding,omitempty"`
	Candidate          string                            `json:"candidate,omitempty"`
	ArtifactID         string                            `json:"artifact_id,omitempty"`
	ExpectedInstanceID string                            `json:"expected_instance_id,omitempty"`
	CheckID            string                            `json:"check_id,omitempty"`
	NativeSession      *contracts.NativeSessionTarget    `json:"native_session,omitempty"`
	Command            string                            `json:"command"`
	RunID              string                            `json:"run_id,omitempty"`
	RequestID          string                            `json:"request_id,omitempty"`
	CommandID          string                            `json:"command_id,omitempty"`
	After              string                            `json:"after,omitempty"`
	Limit              int                               `json:"limit,omitempty"`
	RefreshPR          bool                              `json:"refresh_pr,omitempty"`
}

type wireRequest struct {
	Protocol   string             `json:"protocol"`
	InstanceID string             `json:"instance_id"`
	StateDir   string             `json:"state_dir"`
	Token      string             `json:"token"`
	Options    Options            `json:"options"`
	Request    *contracts.Request `json:"request,omitempty"`
}

type endpoint struct {
	Service contracts.ServiceInfo `json:"service"`
	Pipe    string                `json:"pipe"`
	Token   string                `json:"token"`
}

func DefaultDir() (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		return "", errors.New("LOCALAPPDATA is unavailable; supply an absolute --state-dir")
	}
	return filepath.Join(dir, "SteerRunner"), nil
}

// Normalize resolves existing path aliases, including the existing ancestor of
// a new directory. It does not create files during a read-only CLI operation.
func Normalize(dir string) (string, error) {
	if dir == "" {
		var err error
		dir, err = DefaultDir()
		if err != nil {
			return "", err
		}
	}
	if !filepath.IsAbs(dir) || strings.ContainsAny(dir, "\x00\r\n") {
		return "", errors.New("state-dir must be an absolute path")
	}
	dir = filepath.Clean(dir)
	ancestor := dir
	var missing []string
	for {
		info, err := os.Stat(ancestor)
		if err == nil {
			if !info.IsDir() {
				return "", errors.New("state-dir must have a directory ancestor")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("state-dir has no accessible ancestor")
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	return normalizeCase(filepath.Clean(resolved)), nil
}

func endpointPath(dir string) string { return filepath.Join(dir, "service", "endpoint.json") }

func readEndpoint(dir string) (endpoint, error) {
	var value endpoint
	data, err := os.ReadFile(endpointPath(dir))
	if err != nil {
		return value, err
	}
	pipe, err := pipeName(dir)
	if err != nil {
		return value, err
	}
	if len(data) > 64*1024 || contracts.DecodeStrict(data, &value) != nil || value.Service.StateDir != dir || value.Service.Protocol != contracts.Protocol || value.Pipe != pipe || value.Service.InstanceID == "" || value.Token == "" {
		return value, contracts.Fail(5, "directory_conflict", "Service discovery does not match this state directory.")
	}
	return value, nil
}

func writeEndpoint(dir string, value endpoint) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Join(dir, "service"), "endpoint-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(temporary, endpointPath(dir))
}

func fault(err error, fallback *contracts.Fault) *contracts.Fault {
	var typed *contracts.Fault
	if errors.As(err, &typed) {
		return typed
	}
	return fallback
}

func PublicFault(err error) *contracts.Fault {
	return fault(err, contracts.Fail(4, "service_start_failed", "Service startup failed; inspect the state directory's service log."))
}

type startupFailure struct {
	StartupID string          `json:"startup_id"`
	Exit      int             `json:"exit"`
	Error     contracts.Error `json:"error"`
}

func RecordStartupFailure(dir, startupID string, failure *contracts.Fault) {
	if startupID == "" {
		return
	}
	data, err := json.Marshal(startupFailure{StartupID: startupID, Exit: failure.Exit, Error: failure.Detail})
	if err == nil {
		_ = os.WriteFile(filepath.Join(dir, "service", "startup-"+startupID+".json"), data, 0600)
		completeBootstrap(dir, startupID)
	}
}

func contextAlive(ctx context.Context) bool { return ctx.Err() == nil }
