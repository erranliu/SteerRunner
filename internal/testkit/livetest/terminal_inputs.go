package livetest

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

// TerminalInputFingerprint counts actual original-terminal forwarding intents.
// A missing/empty channel cannot pass a no-additional-input assertion.
func TerminalInputFingerprint(raw []byte, terminal string) (string, error) {
	if terminal == "" {
		return "", errors.New("original terminal missing")
	}
	h := sha256.New()
	count := 0
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var fact struct {
			Kind   string   `json:"kind"`
			CallID string   `json:"call_id"`
			Args   []string `json:"args"`
		}
		if json.Unmarshal(scan.Bytes(), &fact) != nil {
			return "", errors.New("malformed native call evidence")
		}
		if fact.Kind != "intent" || len(fact.Args) < 2 || fact.Args[0] != "terminal" || fact.Args[1] != "send" {
			continue
		}
		bound := ""
		for i, arg := range fact.Args {
			if arg == "--terminal" && i+1 < len(fact.Args) {
				bound = fact.Args[i+1]
			}
		}
		if bound != terminal {
			continue
		}
		if fact.CallID == "" {
			return "", errors.New("native input call identity missing")
		}
		body, _ := json.Marshal(fact)
		h.Write(body)
		count++
	}
	if scan.Err() != nil {
		return "", scan.Err()
	}
	if count == 0 {
		return "", errors.New("zero native inputs cannot prove a stable original input fingerprint")
	}
	return fmt.Sprintf("%d:%x", count, h.Sum(nil)), nil
}
