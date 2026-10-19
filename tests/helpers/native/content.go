package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

func contentReadinessFixture(params map[string]json.RawMessage) (map[string]any, error) {
	var policy struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(params["sandboxPolicy"], &policy)
	var command []string
	_ = json.Unmarshal(params["command"], &command)
	if os.Getenv("STEERRUNNER_CONTENT_FIXTURE") != "1" || policy.Type != "readOnly" || len(command) != 5 {
		return nil, fmt.Errorf("unexpected native readiness command")
	}
	match := regexp.MustCompile(`SR_CONTENT_READY:[a-f0-9]{32}`).FindString(command[4])
	if match == "" {
		return nil, fmt.Errorf("readiness identity missing")
	}
	if err := appendCall(map[string]any{"method": "command/exec", "purpose": "content-readiness", "sandbox": "readOnly"}); err != nil {
		return nil, err
	}
	if exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "content-no-directory")) {
		return map[string]any{"exitCode": 1, "stdout": "", "stderr": "fixture access denied"}, nil
	}
	return map[string]any{"exitCode": 0, "stdout": match, "stderr": ""}, nil
}

func (s *server) releaseContent(ctx context.Context, connection *websocket.Conn) {
	s.mu.Lock()
	session := s.thread.ID
	original := s.thread.Turns[len(s.thread.Turns)-1]
	id := original["id"]
	active := original["status"] == "inProgress"
	s.mu.Unlock()
	if !active {
		return
	}
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			body, err := os.ReadFile(statePath(session))
			var current thread
			if err != nil || json.Unmarshal(body, &current) != nil {
				continue
			}
			for _, turn := range current.Turns {
				if turn["id"] != id || turn["status"] == "inProgress" {
					continue
				}
				_ = connection.Write(ctx, websocket.MessageText, encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": session, "turn": turn}}))
				return
			}
		}
	}
}
