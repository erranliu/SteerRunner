package environmentcheck

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	_ "embed"
	"github.com/erranliu/SteerRunner/internal/contracts"
)

//go:embed windows.ps1
var windowsScript string

//go:embed unix.sh
var unixScript string

type Command struct {
	Executable  string
	Args        []string
	Startup     string
	Shell       string
	Environment map[string]string
}

func Build(r contracts.RunRequest, root, id string, windows, terminal bool) (Command, error) {
	c := r.EnvironmentCheck
	if windows {
		raw, err := windowsInput(r, root)
		if err != nil {
			return Command{}, err
		}
		readInput := "[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + base64.StdEncoding.EncodeToString(raw) + "'))"
		body := windowsBody(readInput, id, settleDelay(terminal))
		units := utf16.Encode([]rune(body))
		encoded := make([]byte, 0, len(units)*2)
		for _, unit := range units {
			encoded = binary.LittleEndian.AppendUint16(encoded, unit)
		}
		data := base64.StdEncoding.EncodeToString(encoded)
		if len(data) > 26000 {
			return Command{}, errors.New("environment invocation exceeds the Windows command budget")
		}
		return Command{Startup: "& powershell.exe -NoProfile -NonInteractive -EncodedCommand $env:STEERRUNNER_ENV_BODY; exit $LASTEXITCODE", Shell: "powershell.exe", Environment: map[string]string{"STEERRUNNER_ENV_BODY": data}}, nil
	}
	if !r.FastForward && c.Interpreter != "bash" {
		return Command{}, errors.New("configured interpreter requires Windows")
	}
	var checks strings.Builder
	if c == nil {
		c = &contracts.EnvironmentCheckInput{}
	}
	paths := make([]string, 0, len(c.Files))
	for name := range c.Files {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		fmt.Fprintf(&checks, "    failure=%s\n    ordinary %s && hash_material %s %s || return 1\n", quote("environment material differs: "+name), quote(name), quote(name), quote(c.Files[name]))
	}
	body := strings.NewReplacer("@@ID@@", id, "@@SETTLE@@", strconv.Itoa(settleDelay(terminal)), "@@ROOT@@", quote(root), "@@REMOTE@@", quote(r.Repository.Remote), "@@URL@@", quote(r.Repository.URL),
		"@@BRANCH@@", quote(r.Repository.BaseBranch), "@@REVISION@@", quote(r.Repository.BaseRevision), "@@FAST_FORWARD@@", strconv.FormatBool(r.FastForward), "@@VERIFY_ONLY@@", strconv.FormatBool(c.VerifyOnly),
		"@@SCRIPT@@", quote(c.Script), "@@MANIFEST@@", quote(c.ManifestSHA256), "@@FENCE@@", strings.Repeat(string(rune(96)), 3), "@@INITIAL@@", quote(r.ResultFile.InitialContentBase64), "@@CHECKS@@", checks.String()).Replace(unixScript)
	// One physical startup line, independent of PTY multiline paste handling.
	data := base64.StdEncoding.EncodeToString([]byte(body))
	startup := "exec bash -c 'if [ \"$(uname -s)\" = Darwin ]; then flag=-D; else flag=-d; fi; exec bash -c \"$(printf %s \"$1\" | base64 \"$flag\")\"' -- \"$STEERRUNNER_ENV_BODY\""
	if len(data) > 26000 {
		return Command{}, errors.New("environment invocation exceeds the terminal command budget")
	}
	return Command{"bash", []string{"-c", body}, startup, "", map[string]string{"STEERRUNNER_ENV_BODY": data}}, nil
}

// PrepareLocalWindows writes the wrapper and its input to a Run-owned directory.
// The caller removes the directory after the managed process tree settles.
func PrepareLocalWindows(r contracts.RunRequest, root, id, dir string) (Command, error) {
	raw, err := windowsInput(r, root)
	if err != nil {
		return Command{}, err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return Command{}, err
	}
	input := filepath.Join(dir, "input.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		return Command{}, err
	}
	script := filepath.Join(dir, "invoke.ps1")
	// A UTF-8 BOM makes Windows PowerShell 5.1 read the wrapper as UTF-8.
	body := "\uFEFFparam([string]$InputPath)\n" + windowsBody("[IO.File]::ReadAllText($InputPath, [Text.UTF8Encoding]::new($false, $true))", id, 0)
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		return Command{}, err
	}
	return Command{Executable: "powershell.exe", Args: []string{"-NoProfile", "-NonInteractive", "-File", script, "-InputPath", input}}, nil
}

func windowsInput(r contracts.RunRequest, root string) ([]byte, error) {
	return json.Marshal(struct {
		*contracts.EnvironmentCheckInput
		Root        string `json:"root"`
		URL         string `json:"url"`
		Remote      string `json:"remote"`
		Branch      string `json:"branch"`
		Revision    string `json:"revision"`
		FastForward bool   `json:"fast_forward"`
		Initial     string `json:"initial"`
	}{r.EnvironmentCheck, root, r.Repository.URL, r.Repository.Remote, r.Repository.BaseBranch, r.Repository.BaseRevision, r.FastForward, r.ResultFile.InitialContentBase64})
}

func windowsBody(readInput, id string, settle int) string {
	return strings.NewReplacer("@@READ_INPUT@@", readInput, "@@ID@@", id, "@@SETTLE@@", strconv.Itoa(settle)).Replace(windowsScript)
}

// Orca discards scrollback on PTY exit. Keep the wrapper alive briefly
// after writing its receipt so Runner can durably save it before exit.
func settleDelay(terminal bool) int {
	if terminal {
		return 15
	}
	return 0
}

func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

type Result struct {
	Data       []byte
	Diagnostic string
	ExitCode   int
}

// Decode accepts only this invocation's bounded result and independent exit code.
var ErrReceiptPending = errors.New("original environment invocation did not produce a completion receipt")

func Decode(lines []string, id string, exitCode *int) (Result, error) {
	prefix := "SLENV-" + id + "-"
	var data strings.Builder
	var result Result
	ended := false
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if ended {
			return Result{}, errors.New("duplicate environment invocation output")
		}
		line = strings.TrimPrefix(line, prefix)
		if chunk, ok := strings.CutPrefix(line, "data:"); ok {
			if len(chunk) == 0 || len(chunk) > 1024 || data.Len()+len(chunk) > base64.StdEncoding.EncodedLen(contracts.MaxResultBytes) {
				return Result{}, errors.New("invalid environment result size")
			}
			data.WriteString(chunk)
			continue
		}
		end, ok := strings.CutPrefix(line, "end:")
		if !ok {
			return Result{}, errors.New("invalid environment invocation output")
		}
		code, diag, ok := strings.Cut(end, ":")
		result.ExitCode, _ = strconv.Atoi(code)
		if !ok || strconv.Itoa(result.ExitCode) != code || exitCode != nil && result.ExitCode != *exitCode {
			return Result{}, errors.New("environment exit status differs from the original process")
		}
		detail, err := base64.StdEncoding.Strict().DecodeString(diag)
		if err != nil || len(detail) > 4096 {
			return Result{}, errors.New("invalid environment invocation diagnostic")
		}
		result.Diagnostic = string(detail)
		ended = true
	}
	if !ended {
		return Result{}, ErrReceiptPending
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(data.String())
	if err != nil || len(raw) > contracts.MaxResultBytes {
		return Result{}, errors.New("invalid environment result bytes")
	}
	result.Data = raw
	return result, nil
}
