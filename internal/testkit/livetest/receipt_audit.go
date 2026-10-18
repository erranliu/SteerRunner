package livetest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

// A title alone never exempts an extra terminal from business-resource counts.
// The frozen helper, exact readonly command, original ready receipt, full wire
// input, terminal incarnation, any returned native receipt and close agree.
type NativeReceiptAuditPolicy struct {
	Executable          string
	SHA256              string
	ConfigurationDigest string
	AccessMode          string
	Environment         string
}

func ReceiptAuditPolicy(m Manifest) NativeReceiptAuditPolicy {
	var s contracts.OrcaSettings
	if contracts.DecodeStrict(m.Execution.Settings, &s) != nil {
		return NativeReceiptAuditPolicy{}
	}
	a := s.AgentLaunch
	mode := s.AccessMode
	if mode == "" {
		mode = "sandbox"
	}
	config := codex.RemoteAgentConfig{Executable: a.Executable, Args: a.Args, Model: a.Model, ReasoningEffort: a.ReasoningEffort, AccessMode: mode, RepositoryURL: RemoteURL}
	if s.GitHubAuth != nil {
		auth := s.GitHubAuth
		config.GitHubAuth = &codex.AuthenticationConfig{AppID: auth.AppID, InstallationID: auth.InstallationID, PrivateKeyEnv: auth.PrivateKeyEnv}
	}
	return NativeReceiptAuditPolicy{Executable: a.RuntimeExecutable, SHA256: a.RuntimeSHA256, AccessMode: mode, Environment: m.Execution.Target.Environment, ConfigurationDigest: codex.RemoteConfigurationDigest(config)}
}

func auditArg(args []string, key string) string {
	value := ""
	for i, a := range args {
		if a == key && i+1 < len(args) {
			if value != "" {
				return ""
			}
			value = args[i+1]
		}
	}
	return value
}
func receiptHash(text string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) }
func sameAuditPath(a, b string) bool {
	return strings.EqualFold(strings.ReplaceAll(a, "\\", "/"), strings.ReplaceAll(b, "\\", "/"))
}
func auditJSON(c nativeAuditCall, runtime string, v any) bool {
	var e struct {
		OK   bool
		Meta struct {
			RuntimeID string `json:"runtimeId"`
		} `json:"_meta"`
		Result json.RawMessage
	}
	return !c.Failed && json.Unmarshal(c.Response, &e) == nil && e.OK && e.Meta.RuntimeID == runtime && json.Unmarshal(e.Result, v) == nil
}

type receiptInspector struct {
	revoker                   bool
	id, handle, inc, worktree string
	query                     codex.RemoteTurnQuery
	read, shown, closed       bool
	invalid                   bool
	lastRead, lastShow        int
}

func onlyReceiptCreateFlags(args []string, p NativeReceiptAuditPolicy) bool {
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		a := args[i]
		if seen[a] {
			return false
		}
		seen[a] = true
		switch a {
		case "--json":
		case "--worktree", "--title", "--command", "--environment":
			i++
			if i >= len(args) || args[i] == "" {
				return false
			}
		default:
			return false
		}
	}
	return seen["--json"] && seen["--worktree"] && seen["--title"] && seen["--command"] && auditArg(args, "--environment") == p.Environment
}

func analyzeReceiptInspectors(calls []nativeAuditCall, runtime string, p NativeReceiptAuditPolicy) (map[string]bool, int, int, int, error) {
	exempt := map[string]bool{}
	if p.Executable == "" || len(p.SHA256) != 64 {
		return exempt, 0, 0, 0, nil
	}
	var ready *codex.RemoteReadiness
	worktree, workspace, originalTerminal, worktreeName := "", "", "", ""
	initialID, initialHandle, initialInc := "", "", ""
	initialShown, initialSole := false, false
	emptyInventory := false
	initialCount := 0
	wireCommands := map[string]string{}
	inspectors := map[string]*receiptInspector{}
	byHandle := map[string]*receiptInspector{}
	pending := map[string][]string{}
	seenReceipts := map[string]bool{}
	for index, c := range calls {
		if len(c.Args) < 2 {
			continue
		}
		action := c.Args[0] + " " + c.Args[1]
		if c.Kind == "intent" {
			if _, exists := pending[c.ID]; exists {
				return exempt, 0, 0, 0, errors.New("duplicate native intent identity")
			}
			pending[c.ID] = c.Args
			if action == "terminal create" && ready == nil && originalTerminal == "" && initialID == "" && worktreeName != "" && emptyInventory && auditArg(c.Args, "--worktree") == "id:"+worktree && auditArg(c.Args, "--title") == "SteerRunner initial execution "+worktreeName && onlyInitialTerminalFlags(c.Args, p) {
				initialID = c.ID
				emptyInventory = false
			}
			if action == "terminal send" {
				if originalTerminal != "" && auditArg(c.Args, "--terminal") != originalTerminal {
					return exempt, 0, 0, 0, errors.New("native input sent to another terminal")
				}
				text := auditArg(c.Args, "--text")
				if strings.HasPrefix(text, "[SteerRunner command=") {
					first, _, ok := strings.Cut(text, "\n")
					if ok {
						parts := strings.Fields(strings.TrimSuffix(first, "]"))
						if len(parts) == 3 {
							command := strings.TrimPrefix(parts[1], "command=")
							if b.StableExecutionID(command) {
								wireCommands[receiptHash(text)] = command
							}
						}
					}
				}
			}
			if action == "terminal create" && ready != nil {
				encoded := auditArg(c.Args, "--command")
				stopPrefix := "& '" + strings.ReplaceAll(p.Executable, "'", "''") + "' '--authentication-stop' '"
				if ready.AuthenticationControl && strings.HasPrefix(encoded, stopPrefix) && strings.HasSuffix(encoded, "'") {
					body, e := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(encoded, stopPrefix), "'"))
					var query codex.RemoteAuthenticationQuery
					if e == nil && contracts.DecodeStrict(body, &query) == nil && query.Session == ready.SessionID && sameAuditPath(query.Workspace, ready.Workspace) && onlyReceiptCreateFlags(c.Args, p) && auditArg(c.Args, "--worktree") == "id:"+worktree && auditArg(c.Args, "--title") == "SteerRunner original authentication revocation" {
						exempt[c.ID] = true
						inspectors[c.ID] = &receiptInspector{id: c.ID, worktree: worktree, revoker: true}
					}
				}
				prefix := "& '" + strings.ReplaceAll(p.Executable, "'", "''") + "' '--original-turn' '"
				if strings.HasPrefix(encoded, prefix) && strings.HasSuffix(encoded, "'") {
					payload := strings.TrimSuffix(strings.TrimPrefix(encoded, prefix), "'")
					body, e := base64.RawURLEncoding.DecodeString(payload)
					var q codex.RemoteTurnQuery
					command := wireCommands[q.InputDigest]
					if e == nil && contracts.DecodeStrict(body, &q) == nil {
						command = wireCommands[q.InputDigest]
					}
					if e == nil && command != "" && onlyReceiptCreateFlags(c.Args, p) && q.Session == ready.SessionID && sameAuditPath(q.Workspace, ready.Workspace) && auditArg(c.Args, "--worktree") == "id:"+worktree && auditArg(c.Args, "--title") == "SteerRunner original native receipt "+command {
						exempt[c.ID] = true
						inspectors[c.ID] = &receiptInspector{id: c.ID, query: q, worktree: worktree}
					}
				}
			}
			continue
		}
		if c.Kind != "receipt" {
			continue
		}
		if c.ID == "" || seenReceipts[c.ID] {
			return exempt, 0, 0, 0, errors.New("missing or duplicate native receipt identity")
		}
		seenReceipts[c.ID] = true
		if args, exists := pending[c.ID]; exists {
			a, _ := json.Marshal(args)
			b, _ := json.Marshal(c.Args)
			if string(a) != string(b) {
				return exempt, 0, 0, 0, errors.New("native receipt command differs from intent")
			}
		} else {
			return exempt, 0, 0, 0, errors.New("native receipt has no original intent")
		}
		if action == "worktree create" {
			var v struct {
				Worktree struct{ ID, Path, DisplayName string }
			}
			if auditJSON(c, runtime, &v) && worktree == "" {
				worktree, workspace = v.Worktree.ID, v.Worktree.Path
				worktreeName = v.Worktree.DisplayName
			}
		}
		if action == "terminal send" {
			var v struct {
				Send struct {
					Handle   string
					Accepted bool
				}
			}
			if !auditJSON(c, runtime, &v) || !v.Send.Accepted || v.Send.Handle != auditArg(c.Args, "--terminal") {
				return exempt, 0, 0, 0, errors.New("native input receipt unavailable or changed")
			}
			if originalTerminal == "" {
				if initialID != "" && (initialHandle == "" || v.Send.Handle != initialHandle) {
					return exempt, 0, 0, 0, errors.New("initial terminal creation differs from original business terminal")
				}
				originalTerminal = v.Send.Handle
			} else if originalTerminal != v.Send.Handle {
				return exempt, 0, 0, 0, errors.New("native input receipt belongs to another terminal")
			}
		}
		if action == "terminal read" && auditArg(c.Args, "--terminal") == originalTerminal {
			var v struct {
				Terminal struct {
					Handle string
					Tail   []string
				}
			}
			if auditJSON(c, runtime, &v) && v.Terminal.Handle == originalTerminal {
				for _, line := range v.Terminal.Tail {
					if !strings.HasPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix) {
						continue
					}
					body, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix))
					var r codex.RemoteReadiness
					if e != nil || json.Unmarshal(body, &r) != nil || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.Repository != RemoteURL || r.RuntimeSHA256 != p.SHA256 || r.ConfigurationDigest != p.ConfigurationDigest || r.AccessMode != p.AccessMode || !sameAuditPath(r.Workspace, workspace) || !codex.ValidNativeID(r.SessionID) || r.Process.PID <= 0 || r.Process.Created == "" {
						return exempt, 0, 0, 0, errors.New("original readiness audit mismatch")
					}
					if ready != nil && !sameReceiptReady(*ready, r) {
						return exempt, 0, 0, 0, errors.New("original readiness changed in native audit")
					}
					if ready == nil && len(wireCommands) > 0 {
						return exempt, 0, 0, 0, errors.New("original readiness appeared after business input")
					}
					ready = &r
				}
			}
		}
		if action == "terminal create" {
			if c.ID == initialID {
				var v struct {
					Terminal struct {
						Handle   string
						Inc      string `json:"incarnationId"`
						Worktree string `json:"worktreeId"`
					}
				}
				if auditJSON(c, runtime, &v) && v.Terminal.Handle != "" && v.Terminal.Inc != "" && v.Terminal.Worktree == worktree {
					initialHandle, initialInc = v.Terminal.Handle, v.Terminal.Inc
				}
			}
			if a := inspectors[c.ID]; a != nil {
				var v struct {
					Terminal struct {
						Handle   string
						Inc      string `json:"incarnationId"`
						Worktree string `json:"worktreeId"`
					}
				}
				if !auditJSON(c, runtime, &v) || v.Terminal.Handle == "" || v.Terminal.Handle == originalTerminal || v.Terminal.Inc == "" || v.Terminal.Worktree != a.worktree {
					a.invalid = true
					continue
				}
				if _, exists := byHandle[v.Terminal.Handle]; exists {
					return exempt, 0, 0, 0, errors.New("readonly inspector handle reused")
				}
				a.handle, a.inc = v.Terminal.Handle, v.Terminal.Inc
				byHandle[a.handle] = a
			}
		}
		if action == "terminal list" && auditArg(c.Args, "--worktree") == "id:"+worktree {
			if initialID == "" {
				emptyInventory = false
			}
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
				if originalTerminal == "" && initialID == "" && len(v.Terminals) == 0 {
					emptyInventory = true
				}
				if initialHandle != "" && len(v.Terminals) == 1 && v.Terminals[0].Handle == initialHandle && v.Terminals[0].Inc == initialInc && v.Terminals[0].Worktree == worktree {
					initialSole = true
				}
				present := map[string]bool{}
				complete := true
				for _, terminal := range v.Terminals {
					if terminal.Worktree != worktree || terminal.Handle == "" || present[terminal.Handle] {
						complete = false
					}
					present[terminal.Handle] = true
					if a := byHandle[terminal.Handle]; a != nil && terminal.Inc != a.inc {
						a.invalid = true
					}
				}
				if complete {
					for handle, a := range byHandle {
						if !present[handle] {
							a.closed = true
						}
					}
				}
			}
		}
		if initialHandle != "" && action == "terminal show" && auditArg(c.Args, "--terminal") == initialHandle {
			var v struct {
				Terminal struct {
					Handle   string
					Inc      string `json:"incarnationId"`
					Worktree string `json:"worktreeId"`
				}
			}
			if !auditJSON(c, runtime, &v) || v.Terminal.Handle != initialHandle || v.Terminal.Inc != initialInc || v.Terminal.Worktree != worktree {
				return exempt, 0, 0, 0, errors.New("initial business terminal incarnation changed")
			}
			initialShown = true
		}
		a := byHandle[auditArg(c.Args, "--terminal")]
		if a == nil {
			continue
		}
		switch action {
		case "terminal send":
			a.invalid = true
		case "terminal show":
			var v struct {
				Terminal struct {
					Handle   string
					Inc      string `json:"incarnationId"`
					Worktree string `json:"worktreeId"`
				}
			}
			if !auditJSON(c, runtime, &v) {
				a.shown = false // A killed reader can be absent; only complete inventory may settle it.
			} else if v.Terminal.Handle != a.handle || v.Terminal.Inc != a.inc || v.Terminal.Worktree != a.worktree {
				a.invalid = true
			} else {
				a.shown = true
				a.lastShow = index
			}
		case "terminal read":
			a.lastRead = index
			var v struct {
				Terminal struct {
					Handle string
					Tail   []string
				}
			}
			if !auditJSON(c, runtime, &v) {
				var e struct {
					Meta struct {
						Runtime string `json:"runtimeId"`
					} `json:"_meta"`
				}
				if json.Unmarshal(c.Response, &e) == nil && e.Meta.Runtime != "" && e.Meta.Runtime != runtime {
					a.invalid = true
				}
				continue
			} // A canceled readonly observation is not a mutation; its close still needs fresh identity proof.
			if !a.shown || v.Terminal.Handle != a.handle {
				a.invalid = true
				continue
			}
			for _, line := range v.Terminal.Tail {
				if a.revoker {
					if !strings.HasPrefix(strings.TrimSpace(line), codex.RemoteAuthenticationPrefix) {
						continue
					}
					body, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(line), codex.RemoteAuthenticationPrefix))
					var proof codex.RemoteAuthenticationCleanup
					if e != nil || json.Unmarshal(body, &proof) != nil || !sameReceiptReady(*ready, proof.Readiness) || !proof.Revoked || !proof.BrokerClosed || !proof.SocketCleaned || proof.Mode != "host-gh-repository-broker" && proof.Mode != "github-app-repository-broker" {
						a.invalid = true
					} else {
						a.read = true
					}
					continue
				}
				if !strings.HasPrefix(strings.TrimSpace(line), codex.RemoteTurnPrefix) {
					continue
				}
				body, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(line), codex.RemoteTurnPrefix))
				var r codex.RemoteTurnReceipt
				if e != nil || json.Unmarshal(body, &r) != nil || r.Source != codex.RemoteTurnSource || r.InputDigest != a.query.InputDigest || !sameReceiptReady(*ready, r.Readiness) || r.Completed && !r.Started || r.Started && (!codex.ValidNativeID(r.NativeTurnID) || r.UserItemID == "") || r.FinalText != "" && (r.FinalItemID == "" || r.FinalDigest != receiptHash(r.FinalText)) {
					a.invalid = true
				} else {
					a.read = true
				}
			}
		case "terminal close":
			var v struct {
				Close struct {
					Handle string
					Killed bool `json:"ptyKilled"`
				}
			}
			if !auditJSON(c, runtime, &v) {
				continue // Lost close receipts require later original identity/absence proof.
			}
			if !a.shown || a.lastShow < a.lastRead || v.Close.Handle != a.handle {
				a.invalid = true
			} else if v.Close.Killed {
				a.closed = true
			}
		}
	}
	closed := 0
	for _, a := range inspectors {
		if a.closed && !a.invalid && (!a.revoker || a.read) {
			closed++
		}
	}
	if initialID != "" && initialHandle != "" && initialInc != "" && initialShown && initialSole && originalTerminal == initialHandle && ready != nil {
		exempt[initialID] = true
		initialCount = 1
	}
	return exempt, len(inspectors), closed, initialCount, nil
}

func onlyInitialTerminalFlags(args []string, p NativeReceiptAuditPolicy) bool {
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		flag := args[i]
		if seen[flag] {
			return false
		}
		seen[flag] = true
		switch flag {
		case "--json":
		case "--title", "--worktree", "--environment":
			i++
			if i >= len(args) || args[i] == "" {
				return false
			}
		default:
			return false
		}
	}
	return seen["--json"] && seen["--title"] && seen["--worktree"] && auditArg(args, "--environment") == p.Environment
}

func sameReceiptReady(a, b codex.RemoteReadiness) bool {
	return a.AuthenticationControl == b.AuthenticationControl && a.Verified == b.Verified && a.NativeTransport == b.NativeTransport && a.RuntimeSHA256 == b.RuntimeSHA256 && a.Contract == b.Contract && a.SessionID == b.SessionID && a.ConfigurationDigest == b.ConfigurationDigest && a.Repository == b.Repository && a.AccessMode == b.AccessMode && a.Process == b.Process && sameAuditPath(a.Workspace, b.Workspace) && a.CheckedAt.Equal(b.CheckedAt) && a.ExpiresAt.Equal(b.ExpiresAt)
}
