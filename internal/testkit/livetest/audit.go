package livetest

import (
	"bytes"
	"encoding/json"
	"errors"
)

type AuditFacts struct {
	Prompts           []string
	Creations         int
	Complete          bool
	Inspections       int
	ClosedInspections int
	InitialTerminals  int
}

type nativeAuditCall struct {
	Kind     string          `json:"kind"`
	ID       string          `json:"call_id"`
	Args     []string        `json:"args"`
	Response json.RawMessage `json:"response"`
	Failed   bool            `json:"failed"`
}

// AnalyzeNativeAudit counts business workspace attempts and separately records
// a verified first terminal when Orca did not create its implicit terminal.
func AnalyzeNativeAudit(raw []byte, runtime string, policies ...NativeReceiptAuditPolicy) (AuditFacts, error) {
	facts := AuditFacts{Complete: true}
	var calls []nativeAuditCall
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var c nativeAuditCall
		if json.Unmarshal(line, &c) != nil {
			return facts, errors.New("invalid native call journal")
		}
		calls = append(calls, c)
	}
	aux := map[string]bool{}
	if len(policies) > 1 {
		return facts, errors.New("multiple native receipt policies")
	}
	if len(policies) == 1 {
		var err error
		aux, facts.Inspections, facts.ClosedInspections, facts.InitialTerminals, err = analyzeReceiptInspectors(calls, runtime, policies[0])
		if err != nil {
			return facts, err
		}
		if facts.Inspections != facts.ClosedInspections {
			facts.Complete = false
		}
	}
	intents := map[string]bool{}
	receipts := map[string]bool{}
	for _, c := range calls {
		if len(c.Args) < 2 {
			continue
		}
		create := c.Args[1] == "create" && (c.Args[0] == "worktree" || c.Args[0] == "terminal")
		send := c.Args[0] == "terminal" && c.Args[1] == "send"
		// The finite fixed-control protocol has its own exact request/response
		// proof above, including native sends whose CLI response was lost.
		if send && aux[c.ID] {
			continue
		}
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
			if create && !aux[c.ID] {
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
