// Package runview publishes a disposable display projection, never execution state.
package runview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Workflow struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type PR struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}
type Run struct {
	ID       string   `json:"run_id"`
	Title    string   `json:"title"`
	Workflow Workflow `json:"workflow"`
	Step     string   `json:"step"`
	Status   string   `json:"status"`
	PR       *PR      `json:"pr"`
}
type Snapshot struct {
	Version   int       `json:"schema_version"`
	Source    string    `json:"source_id"`
	Instance  string    `json:"instance_id"`
	Sequence  uint64    `json:"sequence"`
	Generated time.Time `json:"generated_at"`
	Runs      []Run     `json:"runs"`
}

func Source(normalizedDir string) string {
	sum := sha256.Sum256([]byte(normalizedDir))
	return hex.EncodeToString(sum[:])
}
func Directory() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		return "", errors.New("LOCALAPPDATA unavailable for display export")
	}
	return filepath.Join(root, "SteerRunnerView", "snapshots"), nil
}

// Exporter has one caller: the resident's display goroutine. Failed attempts
// consume a sequence too, including a replacement whose outcome is unknown.
type Exporter struct {
	Directory, Source, Instance string
	Read                        func() ([]Run, error)
	Publish                     func(context.Context, string, []byte) error
	sequence                    uint64
	last                        string
	attempt, published          time.Time
	failed                      bool
}

func (e *Exporter) Tick(ctx context.Context, now time.Time) error {
	if e.failed && now.Sub(e.attempt) < 5*time.Second {
		return nil
	}
	runs, err := e.Read()
	if err != nil {
		return err
	}
	if runs == nil {
		runs = []Run{}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].ID < runs[j].ID })
	content, err := json.Marshal(runs)
	if err != nil {
		return err
	}
	if string(content) == e.last && !e.published.IsZero() && now.Sub(e.published) < 5*time.Second {
		return nil
	}
	e.sequence++
	e.attempt = now
	snapshot := Snapshot{1, e.Source, e.Instance, e.sequence, now.UTC(), runs}
	body, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	publish := e.Publish
	if publish == nil {
		publish = Publish
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = publish(bounded, filepath.Join(e.Directory, e.Source+".json"), append(body, '\n'))
	e.failed = err != nil
	if err == nil {
		e.last = string(content)
		e.published = now
	}
	return err
}

func (e *Exporter) Run(ctx context.Context, diagnostic func(error)) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	var lastDiagnostic time.Time
	for {
		if ctx.Err() != nil {
			return
		}
		if err := e.Tick(ctx, time.Now()); err != nil && time.Since(lastDiagnostic) >= time.Minute {
			diagnostic(err)
			lastDiagnostic = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
