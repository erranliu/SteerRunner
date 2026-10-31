package livetest

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

// A fixed control terminal is exempt only with its frozen command, readiness,
// exact finite requests, original native facts, incarnation and final closure.
func analyzeReceiptInspectors(calls []nativeAuditCall, runtime string, p NativeReceiptAuditPolicy, pending *bool) (map[string]bool, int, int, int, error) {
	exempt := map[string]bool{}
	if p.Executable == "" || len(p.SHA256) != 64 {
		return exempt, 0, 0, 0, nil
	}
	var control *codex.RemoteControlSetup
	var ready *codex.RemoteReadiness
	worktree, workspace, name, original, handle, inc, createID := "", "", "", "", "", "", ""
	initialID, initialHandle, initialInc := "", "", ""
	emptyInventory, initialShown, initialSole, controlReady, closed := false, false, false, false, false
	shown := false
	lastShow, lastRead := -1, -1
	wire := map[string]bool{}
	modelWire := map[string]codex.RemoteModelInput{}
	requests := map[string]codex.RemoteControlRequest{}
	sends := map[string]string{}
	responses := map[string]bool{}
	intents := map[string][]string{}
	receipts := map[string]bool{}
	for index, c := range calls {
		if len(c.Args) < 2 {
			continue
		}
		action := c.Args[0] + " " + c.Args[1]
		terminal := auditArg(c.Args, "--terminal")
		if c.Kind == "intent" {
			if c.ID == "" || intents[c.ID] != nil {
				return exempt, 0, 0, 0, errors.New("duplicate native intent")
			}
			intents[c.ID] = c.Args
			if action == "terminal create" {
				command := auditArg(c.Args, "--command")
				prefix := "& '" + strings.ReplaceAll(p.Executable, "'", "''") + "' '--native-control' '"
				if strings.HasPrefix(command, prefix) && strings.HasSuffix(command, "'") {
					var setup codex.RemoteControlSetup
					raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(command, prefix), "'"))
					if err != nil || contracts.DecodeStrict(raw, &setup) != nil || control != nil || original != "" || len(wire) > 0 || len(setup.Binding) != 64 || setup.RuntimeSHA256 != p.SHA256 || !sameAuditPath(setup.Workspace, workspace) || !onlyReceiptCreateFlags(c.Args, p) || auditArg(c.Args, "--worktree") != "id:"+worktree || auditArg(c.Args, "--title") != "SteerRunner native control" {
						return exempt, 0, 0, 0, errors.New("invalid or late native control creation")
					}
					control = &setup
					createID = c.ID
				} else if emptyInventory && control == nil && original == "" && initialID == "" && name != "" && auditArg(c.Args, "--worktree") == "id:"+worktree && auditArg(c.Args, "--title") == "SteerRunner initial execution "+name && onlyInitialTerminalFlags(c.Args, p) {
					initialID = c.ID
					emptyInventory = false
				}
			}
			if action == "terminal send" {
				text := auditArg(c.Args, "--text")
				if handle != "" && terminal == handle {
					var q codex.RemoteControlRequest
					raw, err := base64.RawURLEncoding.DecodeString(text)
					if !controlReady || closed || ready == nil || err != nil || contracts.DecodeStrict(raw, &q) != nil || q.Validate(*control) != nil || q.Session != ready.SessionID || auditArg(c.Args, "--retry-request") != controlRetryID(q.ID) || !controlSendFlags(c.Args, p) {
						return exempt, 0, 0, 0, errors.New("invalid native control input")
					}
					if q.Method == "original-turn" && !wire[q.InputDigest] {
						return exempt, 0, 0, 0, errors.New("control query has no original business input")
					}
					if old, ok := requests[q.ID]; ok && codex.RemoteControlDigest(old) != codex.RemoteControlDigest(q) {
						return exempt, 0, 0, 0, errors.New("control request ID conflict")
					}
					requests[q.ID] = q
					sends[c.ID] = q.ID
				} else {
					if control != nil && !controlReady {
						return exempt, 0, 0, 0, errors.New("agent started before control readiness")
					}
					if original != "" && terminal != original {
						return exempt, 0, 0, 0, errors.New("input to foreign terminal")
					}
					if original == "" {
						original = terminal
					}
					if strings.HasPrefix(text, "[SteerRunner command=") {
						if ready == nil {
							return exempt, 0, 0, 0, errors.New("business input before original readiness")
						}
						wire[receiptHash(text)] = true
					}
					if strings.HasPrefix(text, "[SteerRunner model=") {
						model, err := codex.ParseRemoteModelPrompt(text)
						if err != nil || ready == nil {
							return exempt, 0, 0, 0, errors.New("invalid original model input")
						}
						wire[receiptHash(text)] = true
						modelWire[receiptHash(text)] = model
					}
				}
			}
			continue
		}
		if c.Kind != "receipt" {
			continue
		}
		if c.ID == "" || receipts[c.ID] || intents[c.ID] == nil {
			return exempt, 0, 0, 0, errors.New("unbound native receipt")
		}
		receipts[c.ID] = true
		a, _ := json.Marshal(intents[c.ID])
		z, _ := json.Marshal(c.Args)
		if string(a) != string(z) {
			return exempt, 0, 0, 0, errors.New("native receipt command changed")
		}
		if action == "worktree create" {
			var v struct {
				Worktree struct{ ID, Path, DisplayName string }
			}
			if auditJSON(c, runtime, &v) && worktree == "" {
				worktree, workspace, name = v.Worktree.ID, v.Worktree.Path, v.Worktree.DisplayName
			}
		}
		if action == "terminal create" && (c.ID == createID || c.ID == initialID) {
			var v struct {
				Terminal struct {
					Handle   string
					Inc      string `json:"incarnationId"`
					Worktree string `json:"worktreeId"`
				}
			}
			if !auditJSON(c, runtime, &v) || v.Terminal.Handle == "" || v.Terminal.Inc == "" || v.Terminal.Worktree != worktree {
				return exempt, 0, 0, 0, errors.New("native terminal creation identity unavailable")
			}
			if c.ID == createID {
				handle, inc = v.Terminal.Handle, v.Terminal.Inc
				if handle == original || handle == initialHandle {
					return exempt, 0, 0, 0, errors.New("control overlaps business terminal")
				}
			} else {
				initialHandle, initialInc = v.Terminal.Handle, v.Terminal.Inc
			}
		}
		if action == "terminal show" && (terminal == handle || terminal == initialHandle) {
			var v struct {
				Terminal struct {
					Handle   string
					Inc      string `json:"incarnationId"`
					Worktree string `json:"worktreeId"`
				}
			}
			if !auditJSON(c, runtime, &v) {
				if terminal == handle {
					shown = false
				}
				continue
			}
			want := inc
			if terminal == initialHandle {
				want = initialInc
			}
			if v.Terminal.Handle != terminal || v.Terminal.Inc != want || v.Terminal.Worktree != worktree {
				return exempt, 0, 0, 0, errors.New("native terminal incarnation changed")
			}
			if terminal == handle {
				shown = true
				lastShow = index
			} else {
				initialShown = true
			}
		}
		if action == "terminal read" && (terminal == handle || terminal == original) {
			var v struct {
				Terminal struct {
					Handle string
					Tail   []string
				}
			}
			if !auditJSON(c, runtime, &v) {
				var envelope struct {
					Meta struct {
						RuntimeID string `json:"runtimeId"`
					} `json:"_meta"`
				}
				if json.Unmarshal(c.Response, &envelope) == nil && envelope.Meta.RuntimeID != "" && envelope.Meta.RuntimeID != runtime {
					return exempt, 0, 0, 0, errors.New("native control read runtime changed")
				}
				continue
			}
			if v.Terminal.Handle != terminal {
				return exempt, 0, 0, 0, errors.New("native read identity changed")
			}
			if terminal == handle && !shown {
				return exempt, 0, 0, 0, errors.New("control read without identity proof")
			}
			if terminal == handle {
				lastRead = index
			}
			for _, line := range v.Terminal.Tail {
				line = strings.TrimSpace(line)
				if terminal == handle && strings.HasPrefix(line, codex.RemoteControlReadyPrefix) {
					var got codex.RemoteControlSetup
					raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteControlReadyPrefix))
					if err != nil || contracts.DecodeStrict(raw, &got) != nil || got != *control {
						return exempt, 0, 0, 0, errors.New("control readiness mismatch")
					}
					controlReady = true
				}
				if terminal == original && strings.HasPrefix(line, codex.RemoteReadyPrefix) {
					var r codex.RemoteReadiness
					raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteReadyPrefix))
					if err != nil || contracts.DecodeStrict(raw, &r) != nil || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.Repository != RemoteURL || r.RuntimeSHA256 != p.SHA256 || r.ConfigurationDigest != p.ConfigurationDigest || r.AccessMode != p.AccessMode || !sameAuditPath(r.Workspace, workspace) || !codex.ValidNativeID(r.SessionID) || r.Process.PID <= 0 || r.Process.Created == "" || ready != nil && !sameReceiptReady(*ready, r) || ready == nil && len(wire) > 0 {
						return exempt, 0, 0, 0, errors.New("original readiness mismatch")
					}
					ready = &r
				}
				if terminal == handle && strings.HasPrefix(line, codex.RemoteControlPrefix) {
					var response codex.RemoteControlResponse
					raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteControlPrefix))
					if err != nil || contracts.DecodeStrict(raw, &response) != nil {
						return exempt, 0, 0, 0, errors.New("invalid control response")
					}
					q, ok := requests[response.ID]
					if !ok || response.Binding != q.Binding || response.RequestDigest != codex.RemoteControlDigest(q) {
						return exempt, 0, 0, 0, errors.New("foreign control response")
					}
					if err := validateAuditControlResponse(q, response, ready, modelWire); err != nil {
						return exempt, 0, 0, 0, err
					}

					responses[q.ID] = true
				}
			}
		}
		if action == "terminal close" && terminal == handle {
			var v struct {
				Close struct {
					Handle string
					Killed bool `json:"ptyKilled"`
				}
			}
			if auditJSON(c, runtime, &v) {
				if !shown || lastShow < lastRead || v.Close.Handle != handle {
					return exempt, 0, 0, 0, errors.New("control close identity mismatch")
				}
				closed = v.Close.Killed
			}
		}
		if action == "terminal list" && auditArg(c.Args, "--worktree") == "id:"+worktree {
			var v struct {
				Terminals []struct {
					Handle   string
					Inc      string `json:"incarnationId"`
					Worktree string `json:"worktreeId"`
				}
				Truncated *bool
				Total     *int `json:"totalCount"`
				Hosts     *struct {
					IDs     []string `json:"hostIds"`
					Omitted []string `json:"omittedHostIds"`
				} `json:"hostScope"`
			}
			if auditJSON(c, runtime, &v) && v.Truncated != nil && !*v.Truncated && v.Total != nil && *v.Total == len(v.Terminals) && v.Hosts != nil && len(v.Hosts.IDs) > 0 && len(v.Hosts.Omitted) == 0 {
				present := map[string]bool{}
				for _, term := range v.Terminals {
					if term.Handle == "" || term.Worktree != worktree || present[term.Handle] {
						return exempt, 0, 0, 0, errors.New("control inventory incomplete")
					}
					present[term.Handle] = true
					if term.Handle == handle && term.Inc != inc {
						return exempt, 0, 0, 0, errors.New("control inventory incarnation changed")
					}
				}
				if original == "" && control == nil && initialID == "" && len(v.Terminals) == 0 {
					emptyInventory = true
				}
				if initialHandle != "" && len(v.Terminals) == 1 && v.Terminals[0].Handle == initialHandle && v.Terminals[0].Inc == initialInc {
					initialSole = true
				}
				if handle != "" && !present[handle] {
					closed = true
				}
			}
		}
	}
	initial := 0
	if initialID != "" && initialShown && initialSole && initialHandle == original && ready != nil {
		exempt[initialID] = true
		initial = 1
	}
	if control == nil {
		return exempt, 0, 0, initial, nil
	}
	if handle == "" || inc == "" || !controlReady || ready == nil || handle == original {
		return exempt, 1, 0, initial, errors.New("fixed control binding incomplete")
	}
	exempt[createID] = true
	for callID, requestID := range sends {
		if !responses[requestID] {
			// Its exact target/payload is a known finite control request, not a
			// business input. Final completeness still requires its response.
			*pending = true
		}
		exempt[callID] = true
	}
	count := 0
	if closed {
		count = 1
	}
	return exempt, 1, count, initial, nil
}

func controlRetryID(id string) string {
	s := receiptHash(id)[:32]
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func controlSendFlags(args []string, p NativeReceiptAuditPolicy) bool {
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		flag := args[i]
		if seen[flag] {
			return false
		}
		seen[flag] = true
		switch flag {
		case "--json", "--enter":
		case "--terminal", "--text", "--retry-request", "--environment":
			i++
			if i >= len(args) || args[i] == "" {
				return false
			}
		default:
			return false
		}
	}
	return seen["--json"] && seen["--enter"] && seen["--terminal"] && seen["--text"] && seen["--retry-request"] && auditArg(args, "--environment") == p.Environment && b.StableExecutionID(strings.ReplaceAll(auditArg(args, "--retry-request"), "-", ""))
}

func validateAuditControlResponse(q codex.RemoteControlRequest, r codex.RemoteControlResponse, ready *codex.RemoteReadiness, models map[string]codex.RemoteModelInput) error {
	if ready == nil {
		return errors.New("original readiness missing")
	}
	if r.Error != "" {
		expected := map[string]string{"workspace-audit": "workspace_audit_unverified", "original-turn": "original_turn_unverified"}[q.Method]
		if r.Error != expected {
			return errors.New("invalid finite control error")
		}
		switch q.Method {
		case "workspace-audit":
			if r.Turn != nil || r.Workspace == nil || !r.Workspace.WritersAbsent {
				return errors.New("workspace reader settlement unverified")
			}
		case "original-turn":
			v := r.Turn
			if r.Workspace != nil || v == nil || !v.InspectionFailed || !v.ReaderWritersAbsent || !sameReceiptReady(*ready, v.Readiness) || v.Source != codex.RemoteTurnSource || v.InputDigest != q.InputDigest {
				return errors.New("failed query reader settlement unverified")
			}
		default:
			return errors.New("unknown control method")
		}
		return nil
	}
	switch q.Method {
	case "workspace-audit":
		if q.Audit == nil || r.Workspace == nil || r.Turn != nil || codex.ValidateRemoteWorkspaceAudit(*ready, *q.Audit, *r.Workspace) != nil {
			return errors.New("original workspace proof mismatch")
		}
	case "original-turn":
		v := r.Turn
		if v == nil || r.Workspace != nil || !v.ReaderWritersAbsent || v.InspectionFailed || !sameReceiptReady(*ready, v.Readiness) || v.Source != codex.RemoteTurnSource || v.InputDigest != q.InputDigest || v.Completed && !v.Started || v.Started && (!codex.ValidNativeID(v.NativeTurnID) || v.UserItemID == "") || v.FinalText != "" && (v.FinalItemID == "" || v.FinalDigest != receiptHash(v.FinalText)) {
			return errors.New("original turn proof mismatch")
		}
		if model, ok := models[q.InputDigest]; ok && v.Started && (v.Model != model.Model || v.ReasoningEffort != model.ReasoningEffort) {
			return errors.New("original native model proof mismatch")
		}
	default:
		return errors.New("unknown finite control method")
	}
	return nil
}
