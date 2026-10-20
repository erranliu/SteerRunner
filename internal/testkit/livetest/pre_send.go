package livetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

type PreSendGate struct {
	CallID     string            `json:"call_id"`
	Terminal   string            `json:"terminal"`
	View       contracts.RunView `json:"view"`
	ReturnedAt time.Time         `json:"returned_at"`
}

func validatePreSendEvidence(m Manifest) error {
	root := filepath.Join(m.Root, "evidence")
	read := func(name string, value any) bool {
		raw, err := os.ReadFile(filepath.Join(root, name))
		return err == nil && json.Unmarshal(raw, value) == nil
	}
	var gate PreSendGate
	var v contracts.RunView
	var intent struct {
		Programs []string `json:"programs"`
	}
	var blocked struct {
		At time.Time `json:"at"`
	}
	audit, err := os.ReadFile(filepath.Join(root, "pre-send-retry-audit.jsonl"))
	if err != nil || !read("pre-send-returned.json", &gate) || !read("pre-send-blocked-run.json", &v) || !read("network-intent.json", &intent) || !read("network-blocked.json", &blocked) || blocked.At.IsZero() || gate.ReturnedAt.Before(blocked.At) || len(intent.Programs) != 1 || !strings.EqualFold(intent.Programs[0], m.RunnerExe) || !PreSendReadRetried(audit, gate) || !PublicationPreSend(v, gate.Terminal) || v.RunID != gate.View.RunID || v.Execution.AttemptID != gate.View.Execution.AttemptID || v.Execution.Generation != gate.View.Execution.Generation || v.Operations[len(v.Operations)-1].ID != gate.View.Operations[len(gate.View.Operations)-1].ID {
		return errors.New("publication pre-send retry lacks original operation/network evidence")
	}
	return nil
}

// Select only the first identity read in a committed publication send. A
// preparation read, readonly inspector or already-attempted send is ineligible.
func PublicationPreSend(v contracts.RunView, terminal string) bool {
	if v.RunID == "" || v.Execution.AttemptID == "" || v.Execution.Generation == "" || v.Step != workflow.StepPublish || v.Status != "running" || v.Execution.TurnStarted || v.ResumeRequired || !v.NewInputsAllowed || len(v.Operations) == 0 || terminal == "" {
		return false
	}
	op := v.Operations[len(v.Operations)-1]
	return op.ID != "" && op.Action == "start_turn" && op.Status == "in_flight" && strings.Contains(v.Execution.SessionID, ":"+terminal+":")
}

// The gate releases a successful original CLI identity read into a Runner-only
// network block. Another original identity read after a full dial timeout proves
// the production readiness loop retried before any terminal/send.
func PreSendReadRetried(raw []byte, gate PreSendGate) bool {
	if !PublicationPreSend(gate.View, gate.Terminal) || gate.CallID == "" || gate.ReturnedAt.IsZero() {
		return false
	}
	var returned bool
	var retried bool
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var c struct {
			ID     string    `json:"call_id"`
			Kind   string    `json:"kind"`
			Args   []string  `json:"args"`
			At     time.Time `json:"at"`
			Ended  time.Time `json:"ended_at"`
			Failed bool      `json:"failed"`
		}
		if json.Unmarshal(line, &c) != nil || len(c.Args) < 4 || c.Args[0] != "terminal" {
			continue
		}
		if c.Args[1] == "send" && !c.At.Before(gate.ReturnedAt) {
			return false
		}
		if c.Args[1] != "show" || auditArg(c.Args, "--terminal") != gate.Terminal {
			continue
		}
		if c.ID == gate.CallID && c.Kind == "receipt" && !c.Failed && !c.Ended.Before(gate.ReturnedAt) {
			returned = true
		}
		if c.ID != gate.CallID && c.ID != "" && c.Kind == "intent" && !c.At.Before(gate.ReturnedAt.Add(5*time.Second)) {
			retried = true
		}
	}
	return returned && retried
}
