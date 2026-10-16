package livetest

import (
	"bytes"
	"encoding/json"
	"errors"
)

type AuditFacts struct {
	Prompts   []string
	Creations int
	Complete  bool
}

// AnalyzeNativeAudit counts actual CLI attempts. Orca worktree.create owns the initial terminal.
func AnalyzeNativeAudit(raw []byte, runtime string) (AuditFacts, error) {
	facts := AuditFacts{Complete: true}
	intents := map[string]bool{}
	receipts := map[string]bool{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var c struct {
			Kind     string          `json:"kind"`
			ID       string          `json:"call_id"`
			Args     []string        `json:"args"`
			Response json.RawMessage `json:"response"`
			Failed   bool            `json:"failed"`
		}
		if json.Unmarshal(line, &c) != nil {
			return facts, errors.New("invalid native call journal")
		}
		if len(c.Args) < 2 {
			continue
		}
		create := c.Args[1] == "create" && (c.Args[0] == "worktree" || c.Args[0] == "terminal")
		send := c.Args[0] == "terminal" && c.Args[1] == "send"
		if !create && !send {
			continue
		}
		if c.ID == "" {
			return facts, errors.New("missing native call identity")
		}
		switch c.Kind {
		case "intent":
			if intents[c.ID] {
				return facts, errors.New("duplicate native intent identity")
			}
			intents[c.ID] = true
			if create {
				facts.Creations++
			}
			if send {
				found := false
				for i, a := range c.Args {
					if a == "--text" && i+1 < len(c.Args) {
						facts.Prompts = append(facts.Prompts, c.Args[i+1])
						found = true
						break
					}
				}
				if !found {
					return facts, errors.New("send has no exact prompt")
				}
			}
		case "receipt":
			var proof struct {
				OK   bool `json:"ok"`
				Meta struct {
					Runtime string `json:"runtimeId"`
				} `json:"_meta"`
			}
			if receipts[c.ID] {
				return facts, errors.New("duplicate native receipt identity")
			}
			receipts[c.ID] = true
			if c.Failed || json.Unmarshal(c.Response, &proof) != nil || !proof.OK || proof.Meta.Runtime != runtime {
				facts.Complete = false
			}
		default:
			return facts, errors.New("native mutation missing journal stage")
		}
	}
	if len(intents) == 0 || len(intents) != len(receipts) {
		facts.Complete = false
	}
	for id := range intents {
		if !receipts[id] {
			facts.Complete = false
		}
	}
	return facts, nil
}
