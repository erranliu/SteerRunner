//go:build windows

package orcatransport

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Exercise the actual Node receipt writer against an OS-held Windows reader.
// A temporary candidate never substitutes for the committed original channel.
func TestNativeReceiptPublicationAcrossWindowsReaders(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node (also required by extension L1) unavailable", err)
	}
	source, err := filepath.Abs(filepath.Join("extensions", "receipt.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, release := range []bool{true, false} {
		name := "held-reader-fails-closed"
		if release {
			name = "reader-closes-publication-recovers"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "original.json")
			if err := os.WriteFile(path, []byte(`{"stage":"started"}`), 0600); err != nil {
				t.Fatal(err)
			}
			reader, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			script := `import {pathToFileURL} from 'node:url'; const {writeReceipt}=await import(pathToFileURL(process.argv[1]).href); console.log('writing'); writeReceipt(process.argv[2],'original',{stage:'settled'});`
			cmd := exec.CommandContext(ctx, node, "--input-type=module", "-e", script, source, dir)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "writing\n" {
				cancel()
				<-done
				t.Fatal("native writer did not start", line, err, stderr.String())
			}
			if release {
				time.Sleep(50 * time.Millisecond)
				_ = reader.Close()
			}
			writeErr := <-done
			var actual struct {
				Stage string `json:"stage"`
			}
			if err := readTerminalFile(dir, "original", &actual); err != nil {
				t.Fatal(err)
			}
			if release {
				if writeErr != nil || actual.Stage != "settled" {
					t.Fatal("brief reader stranded native completion", writeErr, actual, stderr.String())
				}
			} else {
				if writeErr == nil || ctx.Err() != nil || actual.Stage != "started" {
					t.Fatal("uncommitted temporary output became completion or retry exceeded bound", writeErr, ctx.Err(), actual)
				}
				if _, err := os.Stat(path + ".tmp"); err != nil {
					t.Fatal("failed publication did not preserve its evidence", err)
				}
			}
		})
	}
}
