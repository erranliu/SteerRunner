// The helper forwards real native calls and exposes only test-side diagnostics.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
func appendCall(m livetest.Manifest, entry map[string]any) {
	body, err := json.Marshal(entry)
	if err != nil {
		fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(m.Root, "evidence", "native-calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fatal(err)
	}
	_, err = f.Write(append(body, '\n'))
	_ = f.Close()
	if err != nil {
		fatal(err)
	}
}
func main() {
	if len(os.Args) < 2 {
		fatal(fmt.Errorf("mode required"))
	}
	mode := os.Args[1]
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	path := fs.String("manifest", "", "frozen fixture")
	requestFile := fs.String("request", "", "caller request file")
	worktree := fs.String("worktree", "", "original worktree")
	nativeCLI := fs.String("orca", "", "installed local Orca CLI")
	_ = fs.Parse(os.Args[2:])
	raw, err := os.ReadFile(*path)
	if err != nil {
		fatal(err)
	}
	var m livetest.Manifest
	if err = contracts.DecodeStrict(raw, &m); err != nil {
		fatal(err)
	}
	if err = m.Validate(); err != nil {
		fatal(err)
	}
	switch mode {
	case "validate-result":
		raw, err = os.ReadFile(filepath.Join(m.Root, "evidence", "scenario.json"))
		if err != nil {
			fatal(err)
		}
		var r livetest.StabilityResult
		if err = contracts.DecodeStrict(raw, &r); err == nil {
			err = livetest.ValidateEvidence(m, r)
		}
		if err != nil {
			fatal(err)
		}
	case "caller":
		raw, err = os.ReadFile(*requestFile)
		if err != nil {
			fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, m.RunnerExe, "submit", "--input", "-", "--state-dir", filepath.Join(m.Root, "s"), "--json")
		cmd.Stdin = bytes.NewReader(raw)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fatal(fmt.Errorf("submit failed: %w: %s", err, out))
		}
		if err = os.WriteFile(filepath.Join(m.Root, "evidence", "withheld-caller-response.json"), out, 0600); err != nil {
			fatal(err)
		}
		time.Sleep(10 * time.Minute)
		fatal(fmt.Errorf("caller was not terminated at accepted barrier"))
	case "probe-state":
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_, err = native.QueryState(ctx, m.Execution.Target.Environment, m.RemoteRuntimeID, *worktree)
		if err != nil {
			fatal(err)
		}
		fmt.Println(`{"ok":true}`)
	case "probe-subscribe":
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		sub, _, err := native.SubscribeState(ctx, m.Execution.Target.Environment, m.RemoteRuntimeID, *worktree)
		cancel()
		if err != nil {
			fatal(err)
		}
		defer sub.Close()
		_ = os.WriteFile(filepath.Join(m.Root, "evidence", "probe-subscription-open"), []byte("open"), 0600)
		ctx, cancel = context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		for {
			_, err = sub.Next(ctx)
			if err != nil {
				_ = os.WriteFile(filepath.Join(m.Root, "evidence", "probe-subscription-disconnected"), []byte(err.Error()), 0600)
				return
			}
		}
	case "audit", "local-audit":
		args := fs.Args()
		var cmd *exec.Cmd
		if mode == "local-audit" {
			if m.Execution.Target.Kind != "local" || !filepath.IsAbs(*nativeCLI) {
				fatal(fmt.Errorf("local audit requires the frozen installed CLI"))
			}
			cmd = exec.Command(*nativeCLI, args...)
		} else {
			cmd = exec.Command(m.Stability.NativeOrca.Path, append([]string{m.Stability.CLIEntry.Path}, args...)...)
		}
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		started := time.Now().UTC()
		callID := contracts.NewID()
		appendCall(m, map[string]any{"kind": "intent", "call_id": callID, "at": started, "args": args})
		err = cmd.Run()
		if err == nil && m.Scenario == "orca-remote.connection-reconnect" && m.CaseID == "pre-send-disconnect" {
			if gateErr := preSendBarrier(m, args, out.Bytes(), callID); gateErr != nil {
				fatal(gateErr)
			}
		}
		sum := sha256.Sum256([]byte(fmt.Sprint(args)))
		entry := map[string]any{"kind": "receipt", "call_id": callID, "at": started, "ended_at": time.Now().UTC(), "argv_sha256": hex.EncodeToString(sum[:]), "command": args[:min(2, len(args))], "args": args, "response": json.RawMessage(out.Bytes()), "failed": err != nil}
		body, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			entry["response"] = string(out.Bytes())
			body, _ = json.Marshal(entry)
		}
		f, e := os.OpenFile(filepath.Join(m.Root, "evidence", "native-calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			fatal(e)
		}
		_, e = f.Write(append(body, '\n'))
		_ = f.Close()
		if e != nil {
			fatal(e)
		}
		// Test-only response loss: the installed Orca has executed and returned
		// its real receipt. Only this caller's response is withheld; never replay.
		if mode == "local-audit" && err == nil && len(args) > 1 && args[0] == "terminal" && args[1] == "send" {
			text := ""
			for i, arg := range args {
				if arg == "--text" && i+1 < len(args) {
					text = args[i+1]
				}
			}
			if strings.HasPrefix(strings.TrimPrefix(text, "\x1b[200~"), "[SteerRunner command=") {
				marker := filepath.Join(m.Root, "evidence", "drop-next-native-send")
				if e := os.Rename(marker, marker+".consumed"); e == nil {
					fact, _ := json.Marshal(map[string]any{"call_id": callID, "args": args, "response": json.RawMessage(out.Bytes())})
					if e = os.WriteFile(filepath.Join(m.Root, "evidence", "withheld-native-send.json"), fact, 0600); e != nil {
						fatal(e)
					}
					fatal(fmt.Errorf("injected native send response loss after actual Orca receipt"))
				}
			}
		}
		_, _ = os.Stdout.Write(out.Bytes())
		_, _ = os.Stderr.Write(stderr.Bytes())
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				os.Exit(e.ExitCode())
			}
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported helper mode %s", mode))
	}
}

func preSendBarrier(m livetest.Manifest, args []string, output []byte, callID string) error {
	if len(args) < 4 || args[0] != "terminal" || args[1] != "show" {
		return nil
	}
	path := filepath.Join(m.Root, "evidence", "pre-send-gate.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var envelope struct {
		Result struct {
			Terminal native.Terminal `json:"terminal"`
		} `json:"result"`
	}
	if json.Unmarshal(output, &envelope) != nil || envelope.Result.Terminal.Handle == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, m.RunnerExe, "inspect", "--request-id", m.ID+"-submit", "--state-dir", filepath.Join(m.Root, "s"), "--json").Output()
	if err != nil {
		return err
	}
	var view struct {
		Data struct {
			Run contracts.RunView `json:"run"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &view) != nil {
		return fmt.Errorf("invalid pre-send public snapshot")
	}
	if !livetest.PublicationPreSend(view.Data.Run, envelope.Result.Terminal.Handle) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	gate := livetest.PreSendGate{CallID: callID, Terminal: envelope.Result.Terminal.Handle, View: view.Data.Run}
	err = json.NewEncoder(f).Encode(gate)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	until := time.Now().Add(25 * time.Second)
	for time.Now().Before(until) {
		if _, err := os.Stat(filepath.Join(m.Root, "evidence", "pre-send-released")); err == nil {
			gate.ReturnedAt = time.Now().UTC()
			raw, err := json.Marshal(gate)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(m.Root, "evidence", "pre-send-returned.json"), raw, 0600)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("pre-send fault gate timed out; no input released")
}
