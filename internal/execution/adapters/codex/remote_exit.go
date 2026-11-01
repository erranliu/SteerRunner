package codexworker

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type signedRemoteExit struct {
	Body      json.RawMessage `json:"body"`
	Signature []byte          `json:"signature"`
}

func RemoteExitPath(ready RemoteReadiness) (string, error) {
	if len(ready.ExitPublicKey) != ed25519.PublicKeySize {
		return "", errors.New("original Codex exit key missing; PTY exit is insufficient")
	}
	return path.Join(".sr-runtime-stage", "native-stop", hex.EncodeToString(ready.ExitPublicKey), "exit.json"), nil
}

func RemoteStopPath(ready RemoteReadiness) (string, error) {
	p, err := RemoteExitPath(ready)
	return path.Join(path.Dir(p), "request.json"), err
}

// newRemoteStopBinding preserves the public readiness/request identity without
// retaining a signing key or depending on workspace artifact availability.
func newRemoteStopBinding() (ed25519.PublicKey, error) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	clear(key)
	return pub, err
}

func excludeRemoteExit(name string) error {
	const entry = "/.sr-runtime-stage/native-stop/"
	raw, err := os.ReadFile(name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, "\n"+entry+"\n")
	return err
}

// Queries Job accounting while its original handle is still open; an absent
// root PID or closing the handle alone is not a writer-tree proof.
func settleRemoteTree(handle b.StopHandle) bool {
	if handle == nil {
		return true
	}
	handle.Kill()
	tree, ok := handle.(b.ProcessTreeHandle)
	if !ok {
		return false
	}
	until := time.Now().Add(3 * time.Second)
	identity := handle.Identity()
	absent := func() bool { return tree.WritersAbsent() && process.Probe(&identity).State == "absent" }
	for !absent() && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	return absent()
}

// The request has no capability or business input. It can only end this
// helper's already-owned resources and must match this exact readiness.
func awaitRemoteStop(ctx context.Context, ready RemoteReadiness) bool {
	relative, _ := RemoteStopPath(ready)
	name := filepath.Join(ready.Workspace, filepath.FromSlash(relative))
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if f, err := os.Open(name); err == nil {
			raw, e := io.ReadAll(io.LimitReader(f, 8193))
			f.Close()
			if e == nil && len(raw) <= 8192 && bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(encode(ready))) {
				return true
			}
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
