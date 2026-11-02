package orcatransport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestOrcaCLICommandEncodingAndConsumers(t *testing.T) {
	for _, environment := range []string{"", "paired host"} {
		t.Run(environment, func(t *testing.T) {
			l, fake, s := fixtureOn(t, environment)
			profile := l.profile
			profile.Executable, profile.Args = os.Args[0], []string{"prefix", "with space"}
			setup := l.setup.Clone()
			setup.Snapshot.Ref.Target = profile.Target()
			var commands []b.LaunchCommand
			host := &creationTestHost{ManagedProcessHost: fake, output: func(_ context.Context, command b.LaunchCommand) ([]byte, error) {
				commands = append(commands, command)
				return []byte(`{"ok":true,"result":{"receipt":"original"},"_meta":{"runtimeId":"runtime"}}`), nil
			}}
			setup.ProcessHost = host
			setup.LaunchExecutionOwner = func(ctx context.Context, dir string) error { return serveCreationOwner(ctx, dir, host, false) }
			raw, _ := json.Marshal(profile)
			var err error
			setup.ProfileData[0], err = b.NewImplementationDataEnvelope(setup.ProfileData[0], raw)
			if err != nil {
				t.Fatal(err)
			}
			port, err := NewLifecycleWithStateFactory(setup, l.stateFactory)
			if err != nil {
				t.Fatal(err)
			}
			l = port.(*Lifecycle)
			args := []string{"terminal", "create", "--worktree", "id:repo::C:/my workspace", "--command", "Write-Output '中文'", "--json"}
			_, err = l.client.MetaCallWithTimeout(context.Background(), 40*time.Second, args...)
			if err != nil {
				t.Fatal(err)
			}
			key := l.creationKey(s, "operation", "input", "terminal")
			value, err := l.creationCall(context.Background(), key, args, "runtime", "id:repo::C:/my workspace", 40*time.Second, false)
			if err != nil || value.RuntimeID != "runtime" || value.CallUncertain || len(commands) != 2 {
				t.Fatal("consumers did not share original command or receipt", value, err, len(commands))
			}
			// Golden JSON uses the legacy LaunchCommand field order and null
			// EnvironmentRefs. Only machine-specific executable/directory vary.
			executable, _ := json.Marshal(os.Args[0])
			directory, _ := json.Marshal(setup.RepositoryPath)
			route := ""
			if environment != "" {
				route = `,"--environment","paired host"`
			}
			legacy := fmt.Sprintf(`{"Executable":%s,"Args":["prefix","with space","terminal","create","--worktree","id:repo::C:/my workspace","--command","Write-Output '中文'","--json"%s],"WorkingDirectory":%s,"EnvironmentRefs":null,"BlockedEnvironment":["ORCA_ENVIRONMENT","ORCA_PAIRING_CODE","ORCA_REMOTE_PAIRING"],"Timeout":40000000000}`, executable, route, directory)
			for _, command := range commands {
				raw, _ := json.Marshal(command)
				if string(raw) != legacy || digest(raw) != digest([]byte(legacy)) {
					t.Fatal("legacy frozen command bytes/hash changed", string(raw), legacy)
				}
			}
			plan, err := readCreationPlan(creationDirectory(setup.RunDirectory, key))
			if err != nil {
				t.Fatal("original persisted plan rejected", err)
			}
			var legacyCommand b.LaunchCommand
			if err := json.Unmarshal([]byte(legacy), &legacyCommand); err != nil {
				t.Fatal(err)
			}
			plan.Command = legacyCommand
			raw, _ = json.Marshal(plan)
			if err := os.WriteFile(filepath.Join(creationDirectory(setup.RunDirectory, key), CreationOwnerConfigFile), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readCreationPlan(creationDirectory(setup.RunDirectory, key)); err != nil {
				t.Fatal("legacy literal command could not be read", err)
			}
		})
	}
}
