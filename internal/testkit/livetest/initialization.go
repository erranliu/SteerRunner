package livetest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Hide only the caller's native agent identity. The original terminal keeps
// running; retain both actual and delivered envelopes for independent auditing.
func WithholdNativeAgentIdentity(raw []byte) ([]byte, bool, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false, err
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(envelope["result"], &result); err != nil {
		return nil, false, err
	}
	var terminal map[string]json.RawMessage
	if err := json.Unmarshal(result["terminal"], &terminal); err != nil {
		return nil, false, err
	}
	var identity, status string
	var connected bool
	if err := json.Unmarshal(terminal["agentIdentity"], &identity); err != nil {
		return raw, false, nil
	}
	_ = json.Unmarshal(terminal["status"], &status)
	_ = json.Unmarshal(terminal["connected"], &connected)
	if identity != "codex" || !connected || status == "exited" {
		return raw, false, nil
	}
	terminal["agentIdentity"] = json.RawMessage(`""`)
	result["terminal"], _ = json.Marshal(terminal)
	envelope["result"], _ = json.Marshal(result)
	filtered, err := json.Marshal(envelope)
	return filtered, true, err
}

type InitializationState struct {
	Process           b.ProcessIdentity `json:"process"`
	LaunchDigest      string            `json:"launch_digest"`
	LaunchRequest     string            `json:"launch_request"`
	LaunchAttempted   bool              `json:"launch_attempted"`
	LaunchAccepted    bool              `json:"launch_accepted"`
	LaunchConfirmed   bool              `json:"launch_confirmed"`
	Turns             []json.RawMessage `json:"turns"`
	Control           *json.RawMessage  `json:"control"`
	RemoteReadiness   *json.RawMessage  `json:"remote_readiness"`
	RemoteOwnership   *json.RawMessage  `json:"remote_ownership"`
	TerminalOwner     *json.RawMessage  `json:"terminal_owner"`
	TerminalReadiness *json.RawMessage  `json:"terminal_readiness"`
}

func (s InitializationState) NativeOnly() bool {
	return s.Control == nil && s.RemoteReadiness == nil && s.RemoteOwnership == nil && s.TerminalOwner == nil && s.TerminalReadiness == nil
}

type InitializationWindow struct {
	RunID          string              `json:"run_id"`
	Snapshot       b.LifecycleSnapshot `json:"snapshot"`
	NativeTerminal native.Terminal     `json:"native_terminal"`
}

func InitializationResources(s b.LifecycleSnapshot) (InitializationState, error) {
	for _, data := range s.Implementation {
		if data.ImplementationKey == "orca.terminal-state" {
			var state InitializationState
			err := json.Unmarshal(data.Payload, &state)
			return state, err
		}
	}
	return InitializationState{}, errors.New("original initialization resource snapshot missing")
}
func validateInitializationEvidence(m Manifest) error {
	root := filepath.Join(m.Root, "evidence")
	raw, err := os.ReadFile(filepath.Join(root, "initialization-window.json"))
	var window InitializationWindow
	if err != nil || json.Unmarshal(raw, &window) != nil {
		return errors.New("initialization window evidence missing")
	}
	state, err := InitializationResources(window.Snapshot)
	t := window.NativeTerminal
	p := window.Snapshot.Ref.External
	if err != nil || !state.NativeOnly() || state.Process != p || !state.LaunchAttempted || !state.LaunchAccepted || state.LaunchConfirmed || len(state.Turns) != 0 || window.RunID == "" || window.Snapshot.Ref.RunID != window.RunID || t.AgentIdentity != "codex" || !t.Connected || t.Exited() || t.Handle != p.Terminal || t.IncarnationID != p.Incarnation || t.WorktreeID != p.WorktreeID || t.WorktreePath != window.Snapshot.Workspace.Path {
		return errors.New("original native agent did not exist in the frozen pre-ready window")
	}
	var witness struct {
		CallID              string `json:"call_id"`
		Original, Delivered json.RawMessage
	}
	raw, err = os.ReadFile(filepath.Join(root, "agent-identity-withheld.json"))
	if err != nil || json.Unmarshal(raw, &witness) != nil {
		return errors.New("native identity fault missing")
	}
	filtered, hidden, err := WithholdNativeAgentIdentity(witness.Original)
	var actual, expected any
	if err != nil || !hidden || json.Unmarshal(witness.Delivered, &actual) != nil || json.Unmarshal(filtered, &expected) != nil || !reflect.DeepEqual(actual, expected) {
		return errors.New("native identity fault changed unrelated evidence")
	}
	var response struct {
		Result struct{ Terminal native.Terminal }
		Meta   struct {
			RuntimeID string `json:"runtimeId"`
		} `json:"_meta"`
	}
	if json.Unmarshal(witness.Original, &response) != nil || response.Meta.RuntimeID != m.RemoteRuntimeID || !reflect.DeepEqual(response.Result.Terminal, t) {
		return errors.New("native identity witness differs from original source")
	}
	audit, err := os.ReadFile(filepath.Join(root, "native-calls.jsonl"))
	if err != nil {
		return err
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(audit), []byte("\n")) {
		var call nativeAuditCall
		if json.Unmarshal(line, &call) != nil || call.Kind != "receipt" || call.ID != witness.CallID || len(call.Args) < 2 || call.Args[0] != "terminal" || call.Args[1] != "show" || auditArg(call.Args, "--terminal") != p.Terminal {
			continue
		}
		var observed, source any
		if json.Unmarshal(call.Response, &observed) == nil && json.Unmarshal(witness.Original, &source) == nil && reflect.DeepEqual(observed, source) {
			found = true
		}
	}
	if !found {
		return errors.New("native initialization witness is not an original audited response")
	}
	policy := ReceiptAuditPolicy(m)
	facts, err := AnalyzeNativeAudit(audit, m.RemoteRuntimeID, policy)
	if err != nil || policy.NativeLaunchCommand == "" || !facts.Complete || facts.Creations != 1 || facts.Inspections != 0 || len(facts.Prompts) != 1 || facts.Prompts[0] != policy.NativeLaunchCommand || fmt.Sprintf("%x", sha256.Sum256([]byte(facts.Prompts[0]))) != state.LaunchDigest {
		return errors.New("initialization sent business input or accessed helper resources")
	}
	raw, err = os.ReadFile(filepath.Join(root, "offline-settlement.json"))
	var proof contracts.Response
	if err != nil || json.Unmarshal(raw, &proof) != nil || !proof.OK {
		return errors.New("initialization offline settlement missing")
	}
	body, _ := json.Marshal(proof.Data)
	var settled contracts.StoppedVerification
	if json.Unmarshal(body, &settled) != nil || !settled.ResourcesSettled || len(settled.Runs) != 1 || settled.Runs[0].RunID != window.RunID || !settled.Runs[0].ResourcesSettled || settled.Runs[0].StopEvidenceScope != "orca_terminal" {
		return errors.New("initialization offline terminal proof differs")
	}
	if m.CaseID == "restart-before-readiness" {
		raw, err = os.ReadFile(filepath.Join(root, "initialization-restart.json"))
		var restart struct {
			Before, After contracts.ServiceInfo
			Snapshot      b.LifecycleSnapshot
			View          contracts.RunView
		}
		if err != nil || json.Unmarshal(raw, &restart) != nil || restart.Before.InstanceID == "" || restart.After.InstanceID == "" || restart.Before.InstanceID == restart.After.InstanceID || restart.Snapshot.Ref != window.Snapshot.Ref || !restart.View.ResumeRequired || restart.View.NewInputsAllowed {
			return errors.New("initialization replacement/authority evidence missing")
		}
		after, e := InitializationResources(restart.Snapshot)
		if e != nil || !reflect.DeepEqual(after, state) {
			return errors.New("initialization replacement changed original resources")
		}
	}
	return nil
}
