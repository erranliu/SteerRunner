//go:build windows && live

// Package experiments contains explicitly selected, isolated capability trials.
// This trial exercises the real business app-server adapter and frozen audit
// prompt, not a production Run, publication or full takeover qualification.
package experiments

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

var nativeExe = flag.String("native-exe", "", "Existing Codex 0.159.2 executable")
var nativeModel = flag.String("native-model", "gpt-6.1-sol", "Existing account model")
var nativeShell = flag.String("native-shell", "", "Existing unpackaged pwsh.exe; prepended to this trial process PATH only")
var evidenceDir = flag.String("evidence-dir", "", "New private evidence directory")

// Keep the historical baseline explicit after promoting the tested candidate
// prompt. Otherwise a rerun would silently compare the new default to itself.
const baselineAuditPrompt = "审计当前代码，直接修复发现的 P0–P2 问题并完成必要验证。验证后将最终代码改动追加提交到当前分支；无改动时沿用当前提交。报告实际修复的 P0–P2 总数及逐项问题、修复内容；未修复问题和未完成事项如实说明。"

func TestMain(m *testing.M) {
	flag.Parse()
	if *evidenceDir != "" {
		if err := os.MkdirAll(*evidenceDir, 0700); err != nil {
			panic(err)
		}
		log, err := os.OpenFile(filepath.Join(*evidenceDir, "experiment.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		os.Stdout, os.Stderr = log, log
		code := m.Run()
		raw, _ := json.Marshal(map[string]any{"exit_code": code, "finished_at": time.Now().UTC(), "native": *nativeExe, "model": *nativeModel})
		_ = os.WriteFile(filepath.Join(*evidenceDir, "result.json"), raw, 0600)
		_ = log.Close()
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestAuditJSONOutputExperiment(t *testing.T) {
	if *nativeExe == "" || *evidenceDir == "" || *nativeShell == "" {
		t.Skip("explicit isolated experiment opt-in required")
	}
	if !filepath.IsAbs(*nativeExe) || !filepath.IsAbs(*evidenceDir) || !filepath.IsAbs(*nativeShell) || strings.Contains(strings.ToLower(*nativeShell), "windowsapps") {
		t.Fatal("absolute native/evidence paths and an explicit unpackaged shell required")
	}
	version, err := exec.Command(*nativeExe, "--version").Output()
	if err != nil || codex.VerifyPolicyVersion(strings.TrimSpace(string(version))) != nil {
		t.Fatal("native profile not qualified", string(version), err)
	}
	if err := os.MkdirAll(*evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	type trial struct {
		Sample       string `json:"sample"`
		Variant      string `json:"variant"`
		Model        string `json:"model"`
		Session      string `json:"session"`
		Final        string `json:"final"`
		ExtraReports int    `json:"extra_reports"`
		ElapsedMS    int64  `json:"elapsed_ms"`
		FixedCount   int    `json:"fixed_count"`
		PreHead      string `json:"pre_head"`
		PostHead     string `json:"post_head"`
		Clean        bool   `json:"clean"`
	}
	results := []trial{}
	defer func() {
		write(t, filepath.Join(*evidenceDir, "comparison.json"), map[string]any{"scope": "native adapter/prompt trial; not full Run or takeover acceptance", "default_changed": false, "cases": results, "failed": t.Failed()})
	}()
	for _, sample := range []string{"zero", "repair"} {
		for _, variant := range []string{"baseline", "json-override"} {
			t.Run(sample+"/"+variant, func(t *testing.T) {
				root := filepath.Join(*evidenceDir, sample+"-"+variant)
				repo, runtime := filepath.Join(root, "repo"), filepath.Join(root, "runtime")
				if err := os.MkdirAll(repo, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(runtime, 0700); err != nil {
					t.Fatal(err)
				}
				code := "module.exports = values => values.filter(x => Number.isInteger(x) && x > 0).reduce((a, b) => a + b, 0);\n"
				if sample == "repair" {
					code = "module.exports = values => values.reduce((a, b) => a + b, 0);\n"
				}
				checker := "const assert = require('node:assert/strict'); const sum = require('./sum.cjs'); assert.equal(sum([]), 0); assert.equal(sum([3,-5,2,0]), 5); assert.equal(sum([-3]), 0); assert.equal(sum([1,2]), 3); console.log('checks passed');\n"
				if err := os.WriteFile(filepath.Join(repo, "sum.cjs"), []byte(code), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repo, "check.cjs"), []byte(checker), 0600); err != nil {
					t.Fatal(err)
				}
				git(t, repo, "init", "-b", "trial")
				git(t, repo, "config", "user.name", "SteerRunner Trial")
				git(t, repo, "config", "user.email", "trial@steerrunner.invalid")
				git(t, repo, "add", ".")
				git(t, repo, "commit", "-m", "fixed trial input")
				common := filepath.Join(repo, ".git")
				linked := filepath.Join(root, "worktree")
				git(t, repo, "worktree", "add", "-b", "audit", linked, "HEAD")
				repo = linked
				admin := git(t, repo, "rev-parse", "--absolute-git-dir")
				linkedArgs := []string{"--add-dir", filepath.Join(common, "objects"), "--add-dir", filepath.Join(common, "refs"), "--add-dir", filepath.Join(common, "logs"), "--add-dir", admin}
				// The worktree checkout may apply core.autocrlf. Freeze the exact
				// executable checker bytes here, before any model input.
				checkerBytes, err := os.ReadFile(filepath.Join(repo, "check.cjs"))
				if err != nil {
					t.Fatal(err)
				}
				pre := git(t, repo, "rev-parse", "HEAD")
				body := baselineAuditPrompt
				if variant == "json-override" {
					body += "\n最终回复必须为单一 JSON 对象，不添加代码围栏或额外正文。\n" + ra.ResultInstruction("audit")
				}
				// Detached harnesses inherit the OS PATH, whose pwsh may be an
				// MSIX alias. Pin the ordinary OS executable rather than resolving
				// that mutable alias or changing the execution permission mode.
				shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
				if *nativeShell != "" {
					shell = *nativeShell
				}
				if _, err := os.Stat(shell); err != nil {
					t.Fatal(err)
				}
				node, err := exec.LookPath("node")
				if err != nil {
					t.Fatal(err)
				}
				facts := map[string]any{"spec": "sum.cjs exports a function summing only strictly positive integers; input is always an array of integers, empty and all-negative inputs return zero. No other input validation is required. Review implementation against this spec, run the unchanged check.cjs, fix P0-P2 defects if any and commit changes. Do not change check.cjs, create PRs, use network, or add dependencies.", "shell": "For every exec_command explicitly set shell=" + filepath.ToSlash(shell) + " and login=false. Do not use WindowsApps, pwsh from PATH, or Program Files/PowerShell. For validation invoke the absolute Node executable " + filepath.ToSlash(node) + " with check.cjs. Use the actual shell tool, not a simulated Node VM.", "pre_head": pre}
				prompt, err := workflow.RenderPromptWithBody(workflow.StepAuditCode, facts, body)
				if err != nil {
					t.Fatal(err)
				}
				write(t, filepath.Join(root, "frozen-input.json"), map[string]any{"prompt": prompt, "prompt_sha256": ra.Digest([]byte(prompt)), "pre_head": pre, "model": *nativeModel, "effort": "low", "parser_enabled": false})
				host := process.New(10 * time.Second)
				backend := codex.Backend{Config: codex.Config{Executable: *nativeExe, Workspace: repo, RunDir: runtime, RunID: contracts.NewID(), ServerDir: runtime, Sandbox: "workspace-write", WindowsSandbox: "mxc", Model: *nativeModel, ReasoningEffort: "low", WorkerRuntime: true, Environment: func(env []string) []string {
					for i, entry := range env {
						if strings.HasPrefix(strings.ToUpper(entry), "PATH=") {
							env[i] = "PATH=" + filepath.Dir(shell) + string(os.PathListSeparator) + entry[5:]
							return env
						}
					}
					return append(env, "PATH="+filepath.Dir(shell))
				}}, Process: codex.ProcessPort{Start: host.StartManaged, Inspect: host.InspectProcess, NewGeneration: host.NewProcessGeneration, WriteAtomic: host.WriteExecutionFile}}
				var binding codex.Binding
				var owned b.StopHandle
				defer func() {
					if owned != nil {
						owned.Kill()
						if tree, ok := owned.(b.ProcessTreeHandle); ok {
							until := time.Now().Add(5 * time.Second)
							for !tree.WritersAbsent() && time.Now().Before(until) {
								time.Sleep(25 * time.Millisecond)
							}
							if !tree.WritersAbsent() {
								t.Error("native trial writers remain unverified")
							}
						}
						owned.Close()
					}
				}()
				started := time.Now()
				perform := func(text string) codex.Outcome {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
					defer cancel()
					input := b.TurnInput{CommandID: contracts.NewID(), Prompt: text, Model: *nativeModel, ReasoningEffort: "low", LinkedWorktreeArgs: linkedArgs}
					h, fixed, err := backend.PrepareSession(ctx, binding, input)
					if err != nil {
						if h != nil {
							owned = h.Detach()
						}
						t.Fatal("native preparation", err)
					}
					binding = fixed
					paths := h.Paths
					observation, turn, err := backend.StartTurn(ctx, h, input, input.LinkedWorktreeArgs)
					if err != nil {
						owned = h.Detach()
						t.Fatal("native turn", err)
					}
					if owned == nil {
						owned = observation.Process
					}
					defer observation.Cancel()
					for {
						select {
						case event, ok := <-observation.Events:
							if !ok {
								goto completed
							}
							if event.Persisted != nil {
								event.Persisted <- nil
							}
						case <-ctx.Done():
							t.Fatal("native trial timeout; original resources retained in evidence", ctx.Err())
						}
					}
				completed:
					outcome, err := codex.ReadAppServerOutcome(paths.JournalPath, binding.SessionID, turn)
					write(t, filepath.Join(paths.TurnDir, "trial-outcome.json"), outcome)
					if err != nil || !outcome.Succeeded || !outcome.FinalVerified || outcome.LastMessage == "" {
						t.Fatal("native final unverified", err, outcome.Failure)
					}
					return outcome
				}
				outcome := perform(prompt)
				post := git(t, repo, "rev-parse", "HEAD")
				preTree, postTree := git(t, repo, "rev-parse", pre+"^{tree}"), git(t, repo, "rev-parse", post+"^{tree}")
				report, decodeErr := ra.DecodeReport(outcome.LastMessage, "audit")
				extra := 0
				if decodeErr != nil {
					extra++
					outcome = perform("根据原审计上下文整理结果。只报告，不重审、不修代码、不补检查、不提交。最终回复只给一个 JSON 对象。\n" + ra.ResultInstruction("audit"))
					report, decodeErr = ra.DecodeReport(outcome.LastMessage, "audit")
					if post != git(t, repo, "rev-parse", "HEAD") {
						t.Fatal("report changed source")
					}
				}
				if decodeErr != nil || report.Outcome != "done" || report.FixedCount == nil {
					t.Fatal("audit result unavailable", decodeErr)
				}
				want := 0
				if sample == "repair" {
					want = 1
					if preTree == postTree {
						t.Fatal("repair has no tree change")
					}
				} else if preTree != postTree {
					t.Fatal("zero-repair trial changed source")
				}
				if *report.FixedCount != want {
					t.Fatal("repair count differs from seeded defect", *report.FixedCount, want)
				}
				actualChecker, err := os.ReadFile(filepath.Join(repo, "check.cjs"))
				if err != nil || string(actualChecker) != string(checkerBytes) {
					t.Fatal("checker changed")
				}
				if output, err := exec.Command("node", filepath.Join(repo, "check.cjs")).CombinedOutput(); err != nil {
					t.Fatal("independent validation", err, string(output))
				}
				if git(t, repo, "status", "--porcelain") != "" {
					t.Fatal("audit worktree is dirty")
				}
				results = append(results, trial{sample, variant, *nativeModel, binding.SessionID, outcome.LastMessage, extra, time.Since(started).Milliseconds(), want, pre, post, true})
			})
		}
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("git", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}
func write(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		err = os.WriteFile(path, raw, 0600)
	}
	if err != nil {
		t.Error("evidence write", err)
	}
}
