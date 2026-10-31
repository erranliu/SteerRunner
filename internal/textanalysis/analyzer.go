// Package textanalysis owns bounded, disposable Codex CLI text analysis.
// It has no execution lifecycle, business cursor, database or recovery port.
package textanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
	"unicode/utf8"

	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Request struct {
	Text, Instruction string
	Schema            []byte
}
type Analyzer struct {
	mu      sync.Mutex
	active  int
	changed chan struct{}
}

func (a *Analyzer) Analyze(ctx context.Context, in Request, config ra.ParserConfig) (string, error) {
	if len(in.Text) > 65536 || !utf8.ValidString(in.Text) || !utf8.ValidString(in.Instruction) || !json.Valid(in.Schema) {
		return "", errors.New("parser input invalid or exceeds bound")
	}
	prompt, err := inputPrompt(in, config.MaxInputBytes)
	if err != nil {
		return "", err
	}
	var shape any
	if err := json.Unmarshal(in.Schema, &shape); err != nil {
		return "", errors.New("parser schema invalid")
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("urn:steerrunner:text-analysis", shape); err != nil {
		return "", errors.New("parser schema invalid")
	}
	schema, err := compiler.Compile("urn:steerrunner:text-analysis")
	if err != nil {
		return "", errors.New("parser schema unsupported")
	}
	limit := config.MaxConcurrent
	if limit <= 0 {
		limit = 2
	}
	// Admission is local to analysis, independent of Runs and native operations.
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		a.mu.Lock()
		if a.changed == nil {
			a.changed = make(chan struct{})
		}
		available := a.active < limit
		if available {
			a.active++
		}
		changed := a.changed
		a.mu.Unlock()
		if available {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-changed:
		}
	}
	defer func() { a.mu.Lock(); a.active--; close(a.changed); a.changed = make(chan struct{}); a.mu.Unlock() }()
	for attempt := 0; attempt < 2; attempt++ {
		call, cancel := context.WithTimeout(ctx, time.Duration(config.TimeoutSeconds)*time.Second)
		body, err := invoke(call, in, prompt, config)
		cancel()
		if err == nil {
			var value any
			if json.Unmarshal([]byte(body), &value) != nil || schema.Validate(value) != nil {
				return "", errors.New("parser result does not match schema")
			}
			return body, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// Retry only an identified transient provider failure, never tools or bad data.
		if attempt == 1 || err.Error() != "parser_transient_failure" {
			return "", err
		}
	}
	panic("unreachable")
}
