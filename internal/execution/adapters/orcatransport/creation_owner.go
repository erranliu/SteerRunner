package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// Creation owners run on the Runner host, not inside Orca. Their only effect is
// one frozen public CLI create. Service/caller loss cancels observation only.
const CreationOwnerConfigFile = "orca-create-owner.json"

type creationKey struct {
	Project, Execution, Attempt, Generation, Configuration string
	Operation, Input, Scope                                string
}

type creationPlan struct {
	Version       int
	Key           creationKey
	State         string
	Profile       Profile
	Repository    string
	Selector      string
	Runtime       string
	Arguments     []string
	Command       b.LaunchCommand
	ImageSHA256   string
	CommandSHA256 string
	Deadline      time.Time
}

type creationResult struct {
	Version    int
	PlanDigest string
	StartedSHA string
	Output     []byte
	OutputSHA  string
	Failed     bool
}

type creationStarted struct {
	Version    int
	PlanDigest string
	Process    b.ProcessIdentity
}

func (l *Lifecycle) creationKey(s b.LifecycleSnapshot, operation, input, scope string) creationKey {
	return creationKey{s.Session.ProjectID, s.Session.ExecutionID, s.Session.AttemptID, s.ResourceGeneration, s.ConfigurationDigest, operation, input, scope}
}

func creationDirectory(state string, key creationKey) string {
	raw, _ := json.Marshal(key)
	return filepath.Join(state, "orca-creations", digest(raw))
}

func planDigest(plan creationPlan) string {
	raw, _ := json.Marshal(plan)
	return digest(raw)
}

func readCreationPlan(dir string) (creationPlan, error) {
	var plan creationPlan
	raw, err := os.ReadFile(filepath.Join(dir, CreationOwnerConfigFile))
	if err != nil {
		return plan, err
	}
	if len(raw) > 1<<20 || b.DecodeStrictJSON(raw, &plan, "trailing creation plan") != nil || plan.Version != 1 ||
		plan.Key.Project == "" || plan.Key.Execution == "" || plan.Key.Attempt == "" || plan.Key.Generation == "" || plan.Key.Configuration == "" || plan.Key.Operation == "" || plan.Key.Scope == "" ||
		!filepath.IsAbs(plan.State) || filepath.Clean(dir) != creationDirectory(plan.State, plan.Key) ||
		!filepath.IsAbs(plan.Command.Executable) || len(plan.ImageSHA256) != 64 || len(plan.CommandSHA256) != 64 || plan.Deadline.IsZero() || plan.Command.Timeout <= 0 {
		return plan, errors.New("original creation plan is invalid")
	}
	all := append(append([]string(nil), plan.Profile.Args...), plan.Arguments...)
	if plan.Profile.Environment != "" {
		all = append(all, "--environment", plan.Profile.Environment)
	}
	if len(plan.Arguments) < 3 || plan.Arguments[0] != "worktree" && plan.Arguments[0] != "terminal" || plan.Arguments[1] != "create" ||
		!reflect.DeepEqual(plan.Command.Args, all) || plan.Command.WorkingDirectory != plan.Repository ||
		!reflect.DeepEqual(plan.Command.BlockedEnvironment, []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}) || plan.Runtime == "" {
		return plan, errors.New("original creation command is invalid")
	}
	return plan, nil
}

func (l *Lifecycle) originalCreationPlan(key creationKey) (creationPlan, error) {
	plan, err := readCreationPlan(creationDirectory(l.setup.RunDirectory, key))
	if err != nil {
		return plan, err
	}
	if plan.Key != key || !reflect.DeepEqual(plan.Profile, l.profile) || plan.Repository != l.setup.RepositoryPath || plan.State != l.setup.RunDirectory {
		return plan, errors.New("original creation binding changed")
	}
	return plan, nil
}

func (l *Lifecycle) creationCall(ctx context.Context, key creationKey, arguments []string, runtime, selector string, timeout time.Duration, reconcile bool) (native.Result, error) {
	if l.setup.LaunchExecutionOwner == nil {
		if reconcile {
			return native.Result{}, errors.New("original creation has no independent receipt; no create was repeated")
		}
		return l.client.MetaCallWithTimeout(ctx, timeout, arguments...)
	}
	dir := creationDirectory(l.setup.RunDirectory, key)
	plan, err := l.originalCreationPlan(key)
	if os.IsNotExist(err) && !reconcile {
		if err = ctx.Err(); err != nil {
			return native.Result{}, err
		}
		image, imageErr := os.Executable()
		executable, executableErr := exec.LookPath(l.profile.Executable)
		if imageErr != nil || executableErr != nil {
			return native.Result{}, errors.Join(imageErr, executableErr)
		}
		executable, err = filepath.Abs(executable)
		if err != nil {
			return native.Result{}, err
		}
		imageSHA, imageErr := codex.RuntimeDigest(image)
		commandSHA, commandErr := codex.RuntimeDigest(executable)
		if imageErr != nil || commandErr != nil {
			return native.Result{}, errors.Join(imageErr, commandErr)
		}
		all := append(append([]string(nil), l.profile.Args...), arguments...)
		if l.profile.Environment != "" {
			all = append(all, "--environment", l.profile.Environment)
		}
		plan = creationPlan{Version: 1, Key: key, State: l.setup.RunDirectory, Profile: l.profile, Repository: l.setup.RepositoryPath, Selector: selector, Runtime: runtime, Arguments: append([]string(nil), arguments...),
			Command:     b.LaunchCommand{Executable: executable, Args: all, WorkingDirectory: l.setup.RepositoryPath, Timeout: timeout, BlockedEnvironment: []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"}},
			ImageSHA256: imageSHA, CommandSHA256: commandSHA, Deadline: time.Now().UTC().Add(timeout)}
		if err = os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
			return native.Result{}, err
		}
		// An existing directory, including a crash before plan persistence, never
		// authorizes another launch. Only this winner may start the one owner.
		if err = os.Mkdir(dir, 0700); err != nil {
			return native.Result{}, errors.New("original creation launch already claimed; reconcile only")
		}
		raw, _ := json.Marshal(plan)
		if err = l.setup.ProcessHost.WriteExecutionFile(filepath.Join(dir, CreationOwnerConfigFile), raw); err != nil {
			return native.Result{}, err
		}
		if err = l.setup.LaunchExecutionOwner(ctx, dir); err != nil {
			return native.Result{}, fmt.Errorf("original creation owner launch unverified; no replacement: %w", err)
		}
	} else if err != nil {
		return native.Result{}, errors.Join(err, errors.New("original creation receipt unavailable; no create was repeated"))
	}
	if !reflect.DeepEqual(plan.Arguments, arguments) || plan.Runtime != runtime || plan.Selector != selector {
		return native.Result{}, errors.New("original creation command changed")
	}
	// A process failure may leave an intent without a result. It is a finite
	// unknown outcome, not a reason to reset the deadline or run the CLI again.
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, readErr := os.ReadFile(filepath.Join(dir, "create-result.json"))
		if readErr == nil {
			var result creationResult
			var started creationStarted
			startRaw, startErr := os.ReadFile(filepath.Join(dir, "create-started.json"))
			if len(raw) > 24<<20 || b.DecodeStrictJSON(raw, &result, "trailing creation result") != nil || result.Version != 1 || result.PlanDigest != planDigest(plan) || result.OutputSHA != digest(result.Output) ||
				startErr != nil || len(startRaw) > 64<<10 || b.DecodeStrictJSON(startRaw, &started, "trailing creation dispatch") != nil || started.Version != 1 || started.PlanDigest != planDigest(plan) || result.StartedSHA != digest(startRaw) || started.Process.Kind != "os" || started.Process.PID <= 0 || started.Process.Created == "" || !filepath.IsAbs(started.Process.Executable) {
				return native.Result{}, errors.New("original creation receipt invalid or foreign")
			}
			envelope, parseErr := native.ParseEnvelope(result.Output)
			value := native.Result{Payload: envelope.Result, RuntimeID: envelope.Meta.RuntimeID, CallUncertain: result.Failed}
			if envelope.Error != nil {
				value.ErrorCode, value.ErrorMessage = envelope.Error.Code, envelope.Error.Message
			}
			if result.Failed {
				return value, errors.Join(parseErr, errors.New("original creation CLI failed; no create was repeated"))
			}
			return value, parseErr
		}
		if !os.IsNotExist(readErr) {
			return native.Result{}, readErr
		}
		var started creationStarted
		startRaw, startErr := os.ReadFile(filepath.Join(dir, "create-started.json"))
		if startErr == nil && len(startRaw) < 64<<10 && b.DecodeStrictJSON(startRaw, &started, "trailing creation dispatch") == nil && started.Version == 1 && started.PlanDigest == planDigest(plan) && started.Process.Kind == "os" && l.setup.ProcessHost.InspectProcess(&started.Process) == "process absent" {
			return native.Result{}, errors.New("original creation owner exited without a completed receipt; outcome unknown")
		}
		if time.Now().After(plan.Deadline) {
			return native.Result{}, errors.New("original creation has no completed receipt within its original deadline; outcome unknown")
		}
		select {
		case <-ctx.Done():
			return native.Result{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// ServeCreationOwner is a finite, detached Runner process. A durable exclusive
// dispatch marker prevents even a duplicate owner invocation from creating twice.
func ServeCreationOwner(ctx context.Context, dir string) error {
	return serveCreationOwner(ctx, dir, process.New(60*time.Second), true)
}

func serveCreationOwner(ctx context.Context, dir string, host b.ManagedProcessHost, serialize bool) error {
	plan, err := readCreationPlan(dir)
	if err != nil {
		return err
	}
	image, err := os.Executable()
	if err != nil {
		return err
	}
	imageSHA, err := codex.RuntimeDigest(image)
	if err != nil || imageSHA != plan.ImageSHA256 {
		return errors.New("creation owner image changed")
	}
	commandSHA, err := codex.RuntimeDigest(plan.Command.Executable)
	if err != nil || commandSHA != plan.CommandSHA256 {
		return errors.New("original creation CLI image changed")
	}
	id, err := process.CurrentIdentity()
	if err != nil {
		return err
	}
	started := creationStarted{Version: 1, PlanDigest: planDigest(plan), Process: *id}
	marker, err := os.OpenFile(filepath.Join(dir, "create-started.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("original creation dispatch already claimed; no create was repeated")
	}
	raw, _ := json.Marshal(started)
	_, err = marker.Write(raw)
	if err == nil {
		err = marker.Sync()
	}
	closeErr := marker.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err = host.WriteExecutionFile(filepath.Join(dir, "owner-ready.json"), raw); err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(ctx, plan.Deadline)
	defer cancel()
	invoke := func(ctx context.Context) ([]byte, error) { return host.OutputCommand(ctx, plan.Command) }
	var output []byte
	if serialize {
		output, err = native.SerializeCLI(ctx, invoke)
	} else {
		output, err = invoke(ctx)
	}
	result := creationResult{Version: 1, PlanDigest: planDigest(plan), StartedSHA: digest(raw), Output: output, OutputSHA: digest(output), Failed: err != nil}
	raw, _ = json.Marshal(result)
	return host.WriteExecutionFile(filepath.Join(dir, "create-result.json"), raw)
}
