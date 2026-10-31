package nativeaudit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// SandboxAudit exports counts and a digest, never raw native history. Missing
// contexts or tool calls are a failure, not evidence of zero approvals.
type SandboxAudit struct {
	Session     string `json:"session"`
	Contexts    int    `json:"contexts"`
	Tools       int    `json:"tools"`
	Escalations int    `json:"escalations"`
	Approvals   int    `json:"approvals"`
	Bypasses    int    `json:"bypasses"`
	SHA256      string `json:"native_sha256"`
}

func AuditSandboxHistory(source io.Reader, session, workspace string) (SandboxAudit, error) {
	return AuditAccessModeHistory(source, session, workspace, "full_access")
}

func AuditAccessModeHistory(source io.Reader, session, workspace, mode string) (SandboxAudit, error) {
	a := SandboxAudit{Session: session}
	hash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(source, hash))
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	bound := false
	for scanner.Scan() {
		var row struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			return a, errors.New("invalid native history audit source")
		}
		var p map[string]json.RawMessage
		if json.Unmarshal(row.Payload, &p) != nil {
			return a, errors.New("missing native audit payload")
		}
		value := func(name string) string { var s string; _ = json.Unmarshal(p[name], &s); return s }
		switch row.Type {
		case "session_meta":
			if value("id") != session || !strings.EqualFold(strings.ReplaceAll(value("cwd"), "\\", "/"), strings.ReplaceAll(workspace, "\\", "/")) {
				return a, errors.New("native history audit changed session/workspace")
			}
			bound = true
		case "turn_context":
			a.Contexts++
			var policy struct {
				Type    string `json:"type"`
				Network bool   `json:"network_access"`
			}
			expected := "danger-full-access"
			if mode != "full_access" {
				return a, errors.New("invalid audit access mode")
			}
			if json.Unmarshal(p["sandbox_policy"], &policy) != nil || value("approval_policy") != "never" || policy.Type != expected {
				return a, errors.New("native history audit permissions differ from fixed full_access")
			}
		case "response_item":
			kind := value("type")
			if kind != "function_call" && kind != "custom_tool_call" {
				continue
			}
			a.Tools++
			arguments := value("arguments")
			if kind == "custom_tool_call" {
				arguments = value("input")
				if arguments == "" || (value("name") != "exec" && value("name") != "apply_patch") {
					return a, errors.New("unsupported native custom tool permission evidence")
				}
				// Code-mode exec is executable source, not JSON. Conservatively
				// reject any escalation capability in that source (including nested
				// tool calls); never silently omit these actual native tool calls.
				if strings.Contains(arguments, "require_escalated") {
					a.Escalations++
				}
			} else {
				var argumentsObject any
				if json.Unmarshal([]byte(arguments), &argumentsObject) != nil {
					return a, errors.New("native tool arguments are unavailable for permission audit")
				}
				if requestsEscalation(argumentsObject) {
					a.Escalations++
				}
			}
			forbiddenValues := []string{"--dangerously-bypass-approvals-and-sandbox"}

			for _, forbidden := range forbiddenValues {
				if strings.Contains(arguments, forbidden) {
					a.Bypasses++
				}
			}
		case "event_msg":
			if strings.Contains(strings.ToLower(value("type")), "approval") {
				a.Approvals++
			}
		}
	}
	a.SHA256 = hex.EncodeToString(hash.Sum(nil))
	if scanner.Err() != nil {
		return a, scanner.Err()
	}
	if !bound || a.Contexts == 0 || a.Tools == 0 {
		return a, errors.New("native audit evidence incomplete")
	}
	if a.Escalations != 0 || a.Approvals != 0 || a.Bypasses != 0 {
		return a, errors.New("worker requested sandbox escape, approval or bypass")
	}
	return a, nil
}

func requestsEscalation(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if (key == "sandbox_permissions" || key == "sandboxPermissions") && item == "require_escalated" {
				return true
			}
			if requestsEscalation(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if requestsEscalation(item) {
				return true
			}
		}
	}
	return false
}
