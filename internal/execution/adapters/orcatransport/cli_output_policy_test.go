package orcatransport

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const cliOutputFixturePayload = `{"fixture":"original"}`

type cliOutputCall struct {
	command     b.LaunchCommand
	deadline    time.Time
	preparation bool
}

type cliOutputFixture struct {
	client        *native.Client
	frozenSetup   *b.LifecycleSetup
	frozenProfile *Profile
	setup         b.LifecycleSetup
	profile       Profile
	args          []string
	store         string
	calls         []cliOutputCall
}

func writeCLIOutputPairing(t *testing.T, path, environment string, keyByte byte) {
	t.Helper()
	var key [32]byte
	key[0] = keyByte
	data := fmt.Sprintf(`{"environments":[{"id":%q,"preferredEndpointId":"test","endpoints":[{"id":"test","endpoint":"ws://127.0.0.1:1","deviceToken":"fixture-token","publicKeyB64":%q}]}]}`,
		environment, base64.StdEncoding.EncodeToString(key[:]))
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func newCLIOutputFixture(t *testing.T, reader bool, environment string) *cliOutputFixture {
	t.Helper()
	data := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", data)
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	setup, host, profile := fixtureSetup(t, environment)
	setup = setup.Clone()
	profile.Executable = os.Args[0]
	profile.Args = []string{"fixture-prefix", "with space"}
	setup.Snapshot.Ref.Target = profile.Target()

	f := &cliOutputFixture{profile: profile, store: filepath.Join(data, "orca-environments.json")}
	setup.ProcessHost = &creationTestHost{ManagedProcessHost: host, output: func(ctx context.Context, command b.LaunchCommand) ([]byte, error) {
		deadline, _ := ctx.Deadline()
		f.calls = append(f.calls, cliOutputCall{command, deadline, native.PreparationOwnsCLI(ctx)})
		return []byte(`{"ok":true,"result":` + cliOutputFixturePayload + `,"_meta":{"runtimeId":"fixture-runtime"}}`), nil
	}}
	if reader || environment == "" {
		if err := os.WriteFile(f.store, []byte("invalid pairing store"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if reader {
		f.args = []string{"terminal", "list", "--json"}
		setup.Snapshot.Ref.Protocol = b.ProtocolOrcaTerminalV3
		version := ProfileVersion
		if environment != "" {
			version = NativeRemoteProfileVersion
		}
		setup = fixtureProfile(t, setup, ProfileKey, version, profile)
		f.setup = setup.Clone()
		record, err := newFrozenReader(setup)
		if err != nil {
			t.Fatal(err)
		}
		f.client, f.frozenSetup, f.frozenProfile = &record.client, &record.setup, &record.profile
		if _, executes := any(record).(b.LifecyclePort); executes {
			t.Fatal("frozen reader exposes lifecycle execution")
		}
	} else {
		f.args = []string{"status", "--json"}
		setup.Snapshot.Ref.Protocol = b.ProtocolOrcaAgentSessionV1
		agent := AgentSessionProfile{Transport: profile, AccessMode: "full_access"}
		version := AgentSessionVersion
		if environment != "" {
			version = AgentSessionCloseProfileVersion
			writeCLIOutputPairing(t, f.store, environment, 1)
			var err error
			agent.PairingHostID, err = native.UsageHostID(environment)
			if err != nil {
				t.Fatal(err)
			}
		}
		setup = fixtureProfile(t, setup, AgentSessionProfileKey, version, agent)
		f.setup = setup.Clone()
		port, err := NewLifecycle(setup)
		if err != nil {
			t.Fatal(err)
		}
		lifecycle := port.(*Lifecycle)
		f.client, f.frozenSetup, f.frozenProfile = &lifecycle.client, &lifecycle.setup, &lifecycle.profile
	}
	if len(f.calls) != 0 || !reflect.DeepEqual(setup, f.setup) {
		t.Fatal("construction issued a command or changed its input", f.calls)
	}
	return f
}

func (f *cliOutputFixture) requireReply(t *testing.T, result native.Result, err error) cliOutputCall {
	t.Helper()
	if err != nil || result.RuntimeID != "fixture-runtime" || string(result.Payload) != cliOutputFixturePayload || result.CallUncertain || len(f.calls) != 1 {
		t.Fatalf("result = %+v, error = %v, host calls = %d", result, err, len(f.calls))
	}
	return f.calls[0]
}

func TestOrcaCLIOutputConstructorsKeepTransport(t *testing.T) {
	for _, entry := range []struct {
		name   string
		reader bool
	}{{"current", false}, {"frozen-reader", true}} {
		for _, scope := range []struct {
			name        string
			environment string
		}{{"local", ""}, {"paired", "fixture-worker"}} {
			t.Run(entry.name+"/"+scope.name, func(t *testing.T) {
				f := newCLIOutputFixture(t, entry.reader, scope.environment)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				const timeout = 2 * time.Second
				started := time.Now()
				result, err := f.client.MetaCallWithTimeout(ctx, timeout, f.args...)
				returned := time.Now()
				call := f.requireReply(t, result, err)
				args := append([]string{"fixture-prefix", "with space"}, f.args...)
				if scope.environment != "" {
					args = append(args, "--environment", scope.environment)
				}
				want := b.LaunchCommand{
					Executable: f.profile.Executable, Args: args, WorkingDirectory: f.setup.RepositoryPath,
					Timeout: timeout, BlockedEnvironment: []string{"ORCA_ENVIRONMENT", "ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING"},
				}
				parentDeadline, _ := ctx.Deadline()
				if !reflect.DeepEqual(call.command, want) || call.preparation ||
					call.deadline.Before(started.Add(timeout)) || call.deadline.After(returned.Add(timeout)) || !call.deadline.Before(parentDeadline) {
					t.Fatalf("command = %+v, want %+v, deadline = %v, preparation = %t", call.command, want, call.deadline, call.preparation)
				}
				if entry.reader {
					_, _, err := f.client.CloseTerminal(ctx, "fixture-terminal", result.RuntimeID)
					if err == nil || !strings.Contains(err.Error(), "frozen Orca proof does not permit mutation") || len(f.calls) != 1 {
						t.Fatal("reader mutation reached host or lost rejection", err, len(f.calls))
					}
				} else if scope.environment != "" {
					requirePairingRejection := func() {
						t.Helper()
						_, err := f.client.MetaCallWithTimeout(ctx, timeout, f.args...)
						if err == nil || !strings.Contains(err.Error(), "original structured Orca pairing changed or is unavailable") || len(f.calls) != 1 {
							t.Fatal("changed or unavailable pairing reached host or lost rejection", err, len(f.calls))
						}
					}
					writeCLIOutputPairing(t, f.store, scope.environment, 2)
					requirePairingRejection()
					if err := os.Remove(f.store); err != nil {
						t.Fatal(err)
					}
					requirePairingRejection()
				}
				if !reflect.DeepEqual(*f.frozenSetup, f.setup) || !reflect.DeepEqual(*f.frozenProfile, f.profile) {
					t.Fatal("frozen setup or profile changed")
				}
			})
		}
	}
}

func TestOrcaCLIOutputOrdinaryCallWaitsForQueue(t *testing.T) {
	for _, entry := range []struct {
		name   string
		reader bool
	}{{"current", false}, {"frozen-reader", true}} {
		t.Run(entry.name, func(t *testing.T) {
			f := newCLIOutputFixture(t, entry.reader, "")
			holder, cancelHolder := context.WithTimeout(context.Background(), 5*time.Second)
			ready, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			go func() {
				_, err := native.SerializeCLI(holder, func(ctx context.Context) ([]byte, error) {
					close(ready)
					select {
					case <-release:
						return nil, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				})
				done <- err
				close(done)
			}()
			defer func() {
				cancelHolder()
				<-done
			}()
			select {
			case <-ready:
			case err := <-done:
				t.Fatal("lock holder did not start", err)
			case <-holder.Done():
				t.Fatal("lock holder did not become ready", holder.Err())
			}

			wait, cancelWait := context.WithTimeout(context.Background(), 100*time.Millisecond)
			result, err := f.client.MetaCallWithTimeout(wait, time.Second, f.args...)
			cancelWait()
			if !native.IsCLIQueueError(err) || !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, native.Result{}) || len(f.calls) != 0 {
				t.Fatalf("blocked call returned %+v, %v with %d host calls", result, err, len(f.calls))
			}
			close(release)
			if err := <-done; err != nil {
				t.Fatal("lock holder did not release cleanly", err)
			}

			fresh, cancelFresh := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelFresh()
			result, err = f.client.MetaCallWithTimeout(fresh, time.Second, f.args...)
			f.requireReply(t, result, err)
		})
	}
}

func TestOrcaCLIOutputPreparationOwnsCLI(t *testing.T) {
	t.Run("held-lock-and-marker", func(t *testing.T) {
		f := newCLIOutputFixture(t, false, "")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var probeErr error
		checkpoints, competingCalls := 0, 0
		result, err := f.client.PreparationCall(ctx, true, func() error {
			checkpoints++
			// The real preparation caller must already own this same queue key.
			probe, cancelProbe := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancelProbe()
			_, probeErr = native.SerializeCLI(probe, func(context.Context) ([]byte, error) {
				competingCalls++
				return nil, nil
			})
			return nil
		}, f.args...)
		call := f.requireReply(t, result, err)
		if checkpoints != 1 || competingCalls != 0 || !native.IsCLIQueueError(probeErr) || !errors.Is(probeErr, context.DeadlineExceeded) {
			t.Fatalf("checkpoints = %d, competing calls = %d, queue probe error = %v", checkpoints, competingCalls, probeErr)
		}
		parentDeadline, _ := ctx.Deadline()
		if !call.preparation || call.command.Timeout != 15*time.Second || !call.deadline.Equal(parentDeadline) {
			t.Fatalf("preparation = %t, command timeout = %v, deadline = %v, parent deadline = %v", call.preparation, call.command.Timeout, call.deadline, parentDeadline)
		}
	})
	t.Run("checkpoint-rejected", func(t *testing.T) {
		f := newCLIOutputFixture(t, false, "")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rejected := errors.New("fixture checkpoint rejected")
		checkpoints := 0
		result, err := f.client.PreparationCall(ctx, true, func() error {
			checkpoints++
			return rejected
		}, f.args...)
		if checkpoints != 1 || !native.IsCLIQueueError(err) || !errors.Is(err, rejected) || !reflect.DeepEqual(result, native.Result{}) || len(f.calls) != 0 {
			t.Fatalf("rejected checkpoint returned %+v, %v with %d checkpoints and %d host calls", result, err, checkpoints, len(f.calls))
		}
	})
}
