package codexworker

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// Only the original helper holds the signing key. The workspace copy is public
// and untrusted: deletion denies verification, modification cannot grant it.
type RemoteExit struct {
	Readiness            RemoteReadiness    `json:"readiness"`
	TUI                  *b.ProcessIdentity `json:"tui,omitempty"`
	WritersAbsent        bool               `json:"writers_absent"`
	AuthenticationClosed bool               `json:"authentication_closed"`
	ClosedAt             time.Time          `json:"closed_at"`
}

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

func VerifyRemoteExit(raw []byte, ready RemoteReadiness) error {
	if _, err := RemoteExitPath(ready); err != nil {
		return err
	}
	var signed signedRemoteExit
	var exit RemoteExit
	if len(raw) > 64<<10 || b.DecodeStrictJSON(raw, &signed, "trailing native exit") != nil ||
		!ed25519.Verify(ready.ExitPublicKey, signed.Body, signed.Signature) ||
		b.DecodeStrictJSON(signed.Body, &exit, "trailing native exit body") != nil {
		return errors.New("original Codex signed exit is invalid")
	}
	if !bytes.Equal(encode(exit.Readiness), encode(ready)) || !exit.WritersAbsent || !exit.AuthenticationClosed || exit.ClosedAt.Before(ready.CheckedAt) || exit.ClosedAt.IsZero() {
		return errors.New("original Codex writer-tree/authentication exit is incomplete or foreign")
	}
	return nil
}

type remoteExitWriter struct {
	key  ed25519.PrivateKey
	file *os.File
}

func newRemoteExitWriter(workspace string) (*remoteExitWriter, ed25519.PublicKey, error) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	relative, _ := RemoteExitPath(RemoteReadiness{ExitPublicKey: pub})
	name := filepath.Join(workspace, filepath.FromSlash(relative))
	if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return nil, nil, err
	}
	real, err := filepath.EvalSymlinks(filepath.Dir(name))
	if err != nil || !sameWorkspace(real, filepath.Dir(name)) {
		return nil, nil, errors.New("native exit export directory is linked")
	}
	// Open before launching any native worker/model. Retain this exact handle,
	// so later workspace path replacement cannot redirect the final write.
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, nil, err
	}
	return &remoteExitWriter{key: key, file: f}, pub, nil
}

func (w *remoteExitWriter) close() { w.file.Close(); clear(w.key) }

func (w *remoteExitWriter) write(exit RemoteExit) error {
	body := bytes.TrimSpace(encode(exit))
	if _, err := w.file.Write(encode(signedRemoteExit{Body: body, Signature: ed25519.Sign(w.key, body)})); err != nil {
		return err
	}
	return w.file.Sync()
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
