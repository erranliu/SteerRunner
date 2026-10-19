package livetest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Only the test caller's read response changes. The original helper and native
// resources keep running, and the audit retains both original/delivered bytes.
func WithholdReadiness(raw []byte) ([]byte, bool, error) {
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
	var tail []string
	if err := json.Unmarshal(terminal["tail"], &tail); err != nil {
		return nil, false, err
	}
	kept := []string{}
	hidden := false
	for _, line := range tail {
		if strings.HasPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix) {
			hidden = true
			continue
		}
		kept = append(kept, line)
	}
	if !hidden {
		return raw, false, nil
	}
	terminal["tail"], _ = json.Marshal(kept)
	result["terminal"], _ = json.Marshal(terminal)
	envelope["result"], _ = json.Marshal(result)
	filtered, err := json.Marshal(envelope)
	return filtered, true, err
}

type InitializationState struct {
	LaunchDigest    string                 `json:"launch_digest"`
	RemoteOwnership *codex.RemoteOwnership `json:"remote_ownership"`
	RemoteReadiness *codex.RemoteReadiness `json:"remote_readiness"`
	Turns           []json.RawMessage      `json:"turns"`
	LaunchConfirmed bool                   `json:"launch_confirmed"`
}
type InitializationWindow struct {
	RunID           string                `json:"run_id"`
	Snapshot        b.LifecycleSnapshot   `json:"snapshot"`
	NativeReadiness codex.RemoteReadiness `json:"native_readiness"`
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
	policy := ReceiptAuditPolicy(m)
	owner := state.RemoteOwnership
	ready := window.NativeReadiness
	if err != nil || owner == nil || state.RemoteReadiness != nil || state.LaunchConfirmed || len(state.Turns) != 0 || window.RunID == "" || window.Snapshot.Ref.RunID != window.RunID ||
		owner.RuntimeSHA256 != policy.SHA256 || owner.ConfigurationDigest != policy.ConfigurationDigest || ready.Ownership == nil || !reflect.DeepEqual(ready.Ownership, owner) || !ready.Verified || !codex.ValidNativeID(ready.SessionID) || ready.Process.PID <= 0 || ready.Process.Created == "" {
		return errors.New("native resources did not exist in the frozen pre-readiness window")
	}
	var witness struct{ Original, Delivered json.RawMessage }
	raw, err = os.ReadFile(filepath.Join(root, "readiness-withheld.json"))
	if err != nil || json.Unmarshal(raw, &witness) != nil {
		return errors.New("readiness fault was not physically observed")
	}
	filtered, hidden, err := WithholdReadiness(witness.Original)
	var actual, expected any
	if err != nil || !hidden || json.Unmarshal(witness.Delivered, &actual) != nil || json.Unmarshal(filtered, &expected) != nil || !reflect.DeepEqual(actual, expected) {
		return errors.New("readiness response fault changed unrelated evidence")
	}
	// The native ready proof must actually come from the original response.
	var response struct {
		Result struct{ Terminal struct{ Tail []string } }
		Meta   struct {
			RuntimeID string `json:"runtimeId"`
		} `json:"_meta"`
	}
	if json.Unmarshal(witness.Original, &response) != nil || response.Meta.RuntimeID != m.RemoteRuntimeID {
		return errors.New("readiness witness runtime differs")
	}
	found := false
	for _, line := range response.Result.Terminal.Tail {
		if strings.HasPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix) {
			body, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix))
			var r codex.RemoteReadiness
			if e == nil && json.Unmarshal(body, &r) == nil && reflect.DeepEqual(r, ready) {
				found = true
			}
		}
	}
	if !found {
		return errors.New("initialization native readiness lacks original source")
	}
	raw, err = os.ReadFile(filepath.Join(root, "initialization-exit.json"))
	if err != nil || codex.VerifyRemoteOwnershipExit(raw, *owner) != nil {
		return errors.New("original initialization native exit unverified")
	}
	raw, err = os.ReadFile(filepath.Join(root, "offline-settlement.json"))
	var proof contracts.Response
	if err != nil || json.Unmarshal(raw, &proof) != nil || !proof.OK {
		return errors.New("initialization offline resource settlement missing")
	}
	body, _ := json.Marshal(proof.Data)
	var settled contracts.StoppedVerification
	if json.Unmarshal(body, &settled) != nil || !settled.ResourcesSettled || len(settled.Runs) != 1 || settled.Runs[0].RunID != window.RunID || !settled.Runs[0].ResourcesSettled {
		return errors.New("initialization offline proof differs")
	}
	audit, _ := os.ReadFile(filepath.Join(root, "native-calls.jsonl"))
	facts, e := AnalyzeNativeAudit(audit, m.RemoteRuntimeID, policy)
	if e != nil || !facts.Complete || len(facts.Prompts) != 1 || facts.Creations != 1 || fmt.Sprintf("%x", sha256.Sum256([]byte(facts.Prompts[0]))) != state.LaunchDigest || !strings.HasPrefix(facts.Prompts[0], "& '"+strings.ReplaceAll(policy.Executable, "'", "''")+"' '--configuration' '") {
		return errors.New("initialization sent business input or created replacement resources")
	}
	if m.CaseID == "restart-before-readiness" {
		raw, err = os.ReadFile(filepath.Join(root, "initialization-restart.json"))
		var restart struct {
			Before, After contracts.ServiceInfo
			Snapshot      b.LifecycleSnapshot
			View          contracts.RunView
		}
		if err != nil || json.Unmarshal(raw, &restart) != nil || restart.Before.InstanceID == "" || restart.After.InstanceID == "" || restart.Before.InstanceID == restart.After.InstanceID || restart.Snapshot.Ref != window.Snapshot.Ref || !restart.View.ResumeRequired || restart.View.NewInputsAllowed {
			return errors.New("initialization service replacement/authority evidence missing")
		}
		after, e := InitializationResources(restart.Snapshot)
		if e != nil || !reflect.DeepEqual(after, state) {
			return errors.New("initialization replacement changed original resources")
		}
	}
	return nil
}
