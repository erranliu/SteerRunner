package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// A fixed deployment profile for the existing TUI. The dependency installation
// is selected by executable, never downloaded or patched by a Run.
func prepareDSHProfile(ctx context.Context, host process.Host, c TerminalConfig, modules, shell string) error {
	working, err := os.Getwd()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(modules, "@deepseek-harness-tui", "dsh-tui", "package.json"))
	var version struct{ Name, Version string }
	if err != nil || json.Unmarshal(raw, &version) != nil || version.Name != "@deepseek-harness-tui/dsh-tui" || version.Version != "0.16.1" {
		return errors.New("DSH requires the qualified native TUI 0.16.1 installation")
	}
	home := filepath.Join(c.StateDirectory, "home")
	profile := filepath.Join(home, ".dsh", "profiles", "runner")
	if err = os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	write := func(path string, value any) error {
		body, e := json.Marshal(value)
		if e != nil {
			return e
		}
		return host.WriteExecutionFile(path, body)
	}
	if err = write(filepath.Join(profile, "package.json"), map[string]any{"name": "steerrunner-native-dsh", "private": true, "dsh": map[string]any{"profile": map[string]any{"bundles": []string{"@deepseek-ai/dsh-base", "@deepseek-harness-tui/dsh-tui"}}}}); err != nil {
		return err
	}
	if err = write(filepath.Join(profile, "cordis.yml"), []any{}); err != nil {
		return err
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	// A private, read-only dependency reference gives Cordis its native package
	// resolution. The original installation and global profiles are untouched.
	if _, err = host.OutputCommand(ctx, b.LaunchCommand{Executable: shell, Args: []string{"-NoProfile", "-NonInteractive", "-Command", "New-Item -ItemType Junction -Path " + quote(filepath.Join(profile, "node_modules")) + " -Target " + quote(modules) + " -ErrorAction Stop | Out-Null"}}); err != nil {
		return err
	}
	pluginURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(filepath.Join(c.StateDirectory, "dsh-managed.mjs"))}).String()
	model := c.Model
	if model == "" {
		model = "deepseek-flash"
	}
	effort := c.ReasoningEffort
	if effort == "" {
		effort = "low"
	}
	patch := []any{
		map[string]any{"id": "tool-working-directory", "disabled": true},
		map[string]any{"id": "dsh-tui", "config": map[string]any{"provider": "deepseek-official", "model": model, "preset": "runner", "fullscreen": false, "launchpad": false, "effort": effort, "workspace": working, "backend": "dsh"}},
		map[string]any{"insert": []any{
			map[string]any{"id": "steerrunner-preset", "name": "@deepseek-ai/dsh-agent-preset", "config": map[string]any{"id": "runner", "name": "Runner native tools", "plugins": []any{}}},
			map[string]any{"id": "steerrunner-native-evidence", "name": pluginURL},
		}},
	}
	if err = write(filepath.Join(profile, "cordis.patch.yml"), patch); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(home, ".dsh-tui"), 0700); err != nil {
		return err
	}
	return write(filepath.Join(home, ".dsh-tui", "onboarding.json"), map[string]any{"completed": true, "version": 1})
}
