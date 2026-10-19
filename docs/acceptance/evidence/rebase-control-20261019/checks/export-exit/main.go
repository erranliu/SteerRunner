package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"os"
	"path/filepath"
	"time"
)

func main() {
	for _, dir := range os.Args[1:] {
		var saved struct {
			Runtime        string `json:"runtime"`
			Worktree       string `json:"worktree"`
			Authentication struct {
				Proof codex.RemoteAuthenticationCleanup `json:"proof"`
			} `json:"authentication"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, "remote-authentication-cleanup.json"))
		if err != nil {
			panic(err)
		}
		if err = json.Unmarshal(raw, &saved); err != nil {
			panic(err)
		}
		var m livetest.Manifest
		manifestRaw, err := os.ReadFile(filepath.Join(filepath.Dir(dir), "manifest.json"))
		if err == nil {
			err = json.Unmarshal(manifestRaw, &m)
		}
		if err == nil {
			err = m.Validate()
		}
		if err != nil {
			panic(err)
		}
		runtime, err := m.OrcaRuntimeID()
		if err != nil || runtime != saved.Runtime || saved.Worktree == "" {
			panic("original runtime/worktree missing")
		}
		path, err := codex.RemoteExitPath(saved.Authentication.Proof.Readiness)
		if err != nil {
			panic(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		raw, err = native.ReadStopArtifact(ctx, m.Execution.Target.Environment, runtime, saved.Worktree, path)
		cancel()
		if err != nil {
			panic(err)
		}
		if err = codex.VerifyRemoteExit(raw, saved.Authentication.Proof.Readiness); err != nil {
			panic(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "native-exit.json"), raw, 0600); err != nil {
			panic(err)
		}
		digest := sha256.Sum256(raw)
		proof, _ := json.MarshalIndent(map[string]any{"original_artifact_read": true, "runtime": runtime, "worktree": saved.Worktree, "environment": m.Execution.Target.Environment, "path": path, "sha256": hex.EncodeToString(digest[:]), "signature_verified": true, "original_report_reformatted_signature_body": true, "at": time.Now().UTC()}, "", "  ")
		if err = os.WriteFile(filepath.Join(dir, "native-exit-export.json"), append(proof, '\n'), 0600); err != nil {
			panic(err)
		}
		fmt.Println(dir + ": exact original signed bytes recovered and verified")
	}
}
