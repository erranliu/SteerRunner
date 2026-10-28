package process

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Host implements only the small process/file port consumed by execution
// adapters. It has no access to the service or its persistence.
type Host struct{ Timeout time.Duration }

func New(timeout time.Duration) Host { return Host{Timeout: timeout} }

func (host Host) StartManaged(command *exec.Cmd) (executionbackend.StopHandle, error) {
	return StartManaged(command)
}

func (Host) InspectProcess(identity *executionbackend.ProcessIdentity) string {
	return InspectProcess(identity)
}

func (Host) NewProcessGeneration() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}

func (Host) WriteExecutionFile(path string, value []byte) error { return writeAtomic(path, value) }

func (host Host) OutputCommand(ctx context.Context, command executionbackend.LaunchCommand) (result []byte, resultErr error) {
	trace := commandDiagnostic{Started: time.Now().UTC(), Executable: command.Executable, Args: command.Args, Timeout: command.Timeout.String()}
	if deadline, ok := ctx.Deadline(); ok {
		trace.CallerDeadline = &deadline
	}
	output, diagnostics := boundedBuffer{max: 16 << 20}, boundedBuffer{max: 1 << 20}
	if command.DiagnosticPath != "" {
		if err := writeCommandDiagnostic(command.DiagnosticPath+".intent.json", trace); err != nil {
			return nil, fmt.Errorf("command diagnostic intent unavailable; command not invoked: %w", err)
		}
		defer func() {
			trace.Finished = time.Now().UTC()
			trace.Stdout, trace.Stderr = output.String(), diagnostics.String()
			if resultErr != nil {
				trace.Error = resultErr.Error()
			}
			resultErr = errors.Join(resultErr, writeCommandDiagnostic(command.DiagnosticPath+".result.json", trace))
		}()
	}
	if err := ctx.Err(); err != nil {
		at := time.Now().UTC()
		trace.Cancelled = &at
		return nil, err
	}
	timeout := command.Timeout
	if timeout <= 0 {
		timeout = host.Timeout
	}
	trace.Timeout = timeout.String()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if deadline, ok := ctx.Deadline(); ok {
		trace.CommandDeadline = &deadline
	}
	cmd := exec.Command(command.Executable, command.Args...)
	cmd.Dir = command.WorkingDirectory
	if len(command.BlockedEnvironment) > 0 {
		cmd.Env = FilteredEnvironment(command.BlockedEnvironment)
	}
	cmd.Stdout, cmd.Stderr = &output, &diagnostics
	owned, err := host.StartManaged(cmd)
	if err != nil {
		return nil, err
	}
	defer owned.Close()
	trace.Invoked = true
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		at := time.Now().UTC()
		trace.Cancelled = &at
		owned.Kill()
		<-done
		err = ctx.Err()
	}
	code := cmd.ProcessState.ExitCode()
	trace.ExitCode = &code
	if err != nil {
		return output.Bytes(), fmt.Errorf("%s failed: %w (%s)", command.Executable, err, diagnostics.String())
	}
	return output.Bytes(), nil
}

type commandDiagnostic struct {
	Started         time.Time  `json:"started_at"`
	Finished        time.Time  `json:"finished_at,omitempty"`
	CallerDeadline  *time.Time `json:"caller_deadline,omitempty"`
	CommandDeadline *time.Time `json:"command_deadline,omitempty"`
	Cancelled       *time.Time `json:"cancelled_at,omitempty"`
	Executable      string     `json:"executable"`
	Args            []string   `json:"args"`
	Timeout         string     `json:"timeout"`
	Invoked         bool       `json:"invoked"`
	ExitCode        *int       `json:"exit_code,omitempty"`
	Stdout          string     `json:"stdout"`
	Stderr          string     `json:"stderr"`
	Error           string     `json:"error,omitempty"`
}

func writeCommandDiagnostic(path string, value commandDiagnostic) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeAtomic(path, raw)
}

func FilteredEnvironment(names []string) []string {
	environment := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		blocked := false
		for _, name := range names {
			blocked = blocked || strings.EqualFold(key, name)
		}
		if !blocked {
			environment = append(environment, entry)
		}
	}
	return environment
}

type boundedBuffer struct {
	bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > b.max {
		return 0, errors.New("command output exceeds its read limit")
	}
	return b.Buffer.Write(data)
}

func writeAtomic(path string, value []byte) error {
	if path == "" {
		return errors.New("execution file path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".steerline-execution-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(value)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(tmpPath, path)
}
