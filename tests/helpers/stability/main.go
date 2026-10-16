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
	case "audit":
		args := fs.Args()
		cmd := exec.Command(m.Stability.NativeOrca.Path, append([]string{m.Stability.CLIEntry.Path}, args...)...)
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		started := time.Now().UTC()
		callID := contracts.NewID()
		appendCall(m, map[string]any{"kind": "intent", "call_id": callID, "at": started, "args": args})
		err = cmd.Run()
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
