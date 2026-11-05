// Package metadataprocess owns disposable, precisely scoped native processes.
package metadataprocess

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Process struct {
	Input    io.WriteCloser
	Output   io.ReadCloser
	cmd      *exec.Cmd
	stop     func() error
	done     chan struct{}
	close    sync.Once
	closeErr error
}

var ErrCleanup = errors.New("metadata cleanup unverified")

// Executable excludes the caller's working directory from PATH resolution.
func Executable(program string) (string, error) {
	if filepath.IsAbs(program) {
		return exec.LookPath(program)
	}
	if program == "" || strings.ContainsAny(program, `/\\:`) {
		return "", errors.New("relative metadata executable is unsupported")
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(dir) {
			continue
		}
		if path, err := exec.LookPath(filepath.Join(dir, program)); err == nil && filepath.IsAbs(path) {
			return path, nil
		}
	}
	return "", errors.New("native metadata executable unavailable")
}

func Open(ctx context.Context, executable string, args ...string) (*Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir, _ = os.UserHomeDir()
	cmd.WaitDelay = 5 * time.Second
	p := &Process{cmd: cmd, done: make(chan struct{})}
	var err error
	if p.Input, err = cmd.StdinPipe(); err != nil {
		return nil, err
	}
	if p.Output, err = cmd.StdoutPipe(); err != nil {
		p.Input.Close()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if p.stop, err = StartOwned(cmd); err != nil {
		p.Input.Close()
		p.Output.Close()
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			p.Close()
		case <-p.done:
		}
	}()
	return p, nil
}

func (p *Process) Close() error {
	p.close.Do(func() {
		p.Input.Close()
		if err := p.stop(); err != nil {
			p.closeErr = errors.Join(ErrCleanup, err)
		}
		go func() { _ = p.cmd.Wait(); close(p.done) }()
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			p.closeErr = ErrCleanup
		}
		p.Output.Close()
	})
	return p.closeErr
}
