package contracts

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const EnvironmentCheckContract = "steerrunner-environment-check/v1"

type CheckInput struct {
	Correlation      map[string]string `json:"correlation,omitempty"`
	Repository       Repository        `json:"repository"`
	Execution        Execution         `json:"execution"`
	Publication      *Publication      `json:"publication,omitempty"`
	ExecutionTimeout string            `json:"execution_timeout,omitempty"`
	CleanupTimeout   string            `json:"cleanup_timeout,omitempty"`
}

type CheckStopInput struct {
	CheckID          string `json:"check_id"`
	ExpectedRevision uint64 `json:"expected_revision"`
}

type CheckReceipt struct {
	CommandID     string  `json:"command_id"`
	CheckID       *string `json:"check_id"`
	ReceiptStatus string  `json:"receipt_status"`
	ActionStatus  string  `json:"action_status"`
}

type CheckCommand struct {
	RequestID     string  `json:"request_id"`
	Command       string  `json:"command"`
	CommandID     string  `json:"command_id"`
	CheckID       *string `json:"check_id"`
	ReceiptStatus string  `json:"receipt_status"`
	ActionStatus  string  `json:"action_status"`
	Error         *Error  `json:"error"`
}

type CheckInspection struct {
	Command *CheckCommand     `json:"command"`
	Check   *EnvironmentCheck `json:"check"`
}

type CheckEvidence struct {
	CheckID     string            `json:"check_id"`
	SessionID   string            `json:"session_id"`
	TurnID      string            `json:"turn_id"`
	Nonce       string            `json:"nonce"`
	OperationID string            `json:"operation_id"`
	ToolCallID  string            `json:"tool_call_id"`
	Source      string            `json:"source"`
	Verified    bool              `json:"verified"`
	Facts       map[string]string `json:"facts,omitempty"`
}

type CheckItem struct {
	ID       string          `json:"id"`
	Required bool            `json:"required"`
	Status   string          `json:"status"`
	Stage    string          `json:"stage"`
	Evidence []CheckEvidence `json:"evidence"`
}

type CheckContext struct {
	RuntimeID            string `json:"runtime_id,omitempty"`
	Terminal             string `json:"terminal,omitempty"`
	TerminalIncarnation  string `json:"terminal_incarnation,omitempty"`
	WorktreeID           string `json:"worktree_id,omitempty"`
	Nonce                string `json:"nonce,omitempty"`
	NativeGeneration     string `json:"native_generation,omitempty"`
	AttemptID            string `json:"attempt_id"`
	Generation           string `json:"generation"`
	SessionID            string `json:"session_id,omitempty"`
	TurnID               string `json:"turn_id,omitempty"`
	RuntimeVersion       string `json:"runtime_version,omitempty"`
	PID                  int    `json:"pid,omitempty"`
	ProcessCreated       string `json:"process_created,omitempty"`
	Account              string `json:"account,omitempty"`
	AuthenticationSource string `json:"authentication_source,omitempty"`
	AccessMode           string `json:"access_mode"`
	Sandbox              string `json:"sandbox,omitempty"`
	ApprovalPolicy       string `json:"approval_policy,omitempty"`
	Workspace            string `json:"workspace,omitempty"`
}

type CheckResources struct {
	BaseOwned     bool   `json:"base_owned"`
	HeadOwned     bool   `json:"head_owned"`
	BaseRef       string `json:"base_ref"`
	HeadRef       string `json:"head_ref"`
	BaseSHA       string `json:"base_sha"`
	HeadSHA       string `json:"head_sha,omitempty"`
	PRURL         string `json:"pr_url,omitempty"`
	PRNumber      int    `json:"pr_number,omitempty"`
	PRClosed      bool   `json:"pr_closed"`
	BaseDeleted   bool   `json:"base_deleted"`
	HeadDeleted   bool   `json:"head_deleted"`
	WriterStopped bool   `json:"writer_stopped"`
	LocalRemoved  bool   `json:"local_removed"`
}

type EnvironmentCheck struct {
	CheckID           string         `json:"check_id"`
	Revision          uint64         `json:"revision"`
	Contract          string         `json:"contract"`
	InputDigest       string         `json:"input_digest"`
	ResourceDigest    string         `json:"resource_digest"`
	Input             CheckInput     `json:"input"`
	Actual            CheckContext   `json:"actual"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	ExecutionDeadline time.Time      `json:"execution_deadline"`
	CleanupDeadline   time.Time      `json:"cleanup_deadline"`
	Status            string         `json:"status"`
	Result            string         `json:"result"`
	CleanupStatus     string         `json:"cleanup_status"`
	Diagnostic        string         `json:"diagnostic,omitempty"`
	Items             []CheckItem    `json:"items"`
	Resources         CheckResources `json:"resources"`
	AllowedActions    []string       `json:"allowed_actions"`
	Excluded          []string       `json:"excluded"`
}

func CheckDurations(value CheckInput) (time.Duration, time.Duration, error) {
	execution, cleanup := 10*time.Minute, 2*time.Minute
	var err error
	if value.ExecutionTimeout != "" {
		execution, err = time.ParseDuration(value.ExecutionTimeout)
		if err != nil {
			return 0, 0, err
		}
	}
	if value.CleanupTimeout != "" {
		cleanup, err = time.ParseDuration(value.CleanupTimeout)
	}
	return execution, cleanup, err
}

func ValidateEnvironmentCheck(data []byte) *Fault {
	invalid := Fail(2, "invalid_input", "Environment check input does not satisfy the embedded contract.")
	var value CheckInput
	if DecodeStrict(data, &value) != nil || !safeURL(value.Repository.URL) || !commitPattern.MatchString(value.Repository.BaseRevision) || !plain(value.Repository.Remote) || !plain(value.Repository.BaseBranch) || strings.HasPrefix(value.Repository.Remote, "-") || strings.HasPrefix(value.Repository.BaseBranch, "-") {
		return invalid
	}
	if value.Repository.LocalPath != "" && !filepath.IsAbs(value.Repository.LocalPath) {
		return invalid
	}
	for key := range value.Correlation {
		if secretArgument.MatchString(key) {
			return invalid
		}
	}
	execution, cleanup, err := CheckDurations(value)
	if err != nil || execution <= 0 || execution > time.Hour || cleanup <= 0 || cleanup > 10*time.Minute {
		return invalid
	}
	if err := ValidateExecution(value.Execution); err != nil {
		return err
	}
	if value.Publication != nil {
		return ValidatePublication(value.Repository, *value.Publication)
	}
	return nil
}

func CheckManifest(publication bool) []CheckItem {
	items := []CheckItem{}
	for _, id := range []string{"permissions", "workspace", "temporary", "git_admin", "remote_read", "fetch", "base_push", "commit", "head_push", "pr_create", "pr_query", "pr_close"} {
		required := publication || !strings.HasPrefix(id, "pr_")
		status := "unverified"
		if !required {
			status = "not_applicable"
		}
		items = append(items, CheckItem{ID: id, Required: required, Status: status, Stage: "not_started", Evidence: []CheckEvidence{}})
	}
	return items
}

// Missing/empty manifests, foreign evidence and model prose never prove readiness.
func CheckResult(check EnvironmentCheck) string {
	manifest := CheckManifest(check.Input.Publication != nil)
	if len(check.Items) != len(manifest) {
		return "unverified"
	}
	result := "passed"
	for i, item := range check.Items {
		if item.ID != manifest[i].ID || item.Required != manifest[i].Required {
			return "unverified"
		}
		if !item.Required {
			if item.Status != "not_applicable" {
				return "unverified"
			}
			continue
		}
		if item.Status == "failed" {
			result = "failed"
			continue
		}
		if item.Status != "passed" || len(item.Evidence) == 0 {
			if result != "failed" {
				result = "unverified"
			}
			continue
		}
		for _, e := range item.Evidence {
			if !e.Verified || e.CheckID != check.CheckID || e.SessionID == "" || e.SessionID != check.Actual.SessionID || e.TurnID == "" || e.TurnID != check.Actual.TurnID || e.Nonce == "" || e.Nonce != check.Actual.Nonce || e.OperationID == "" || e.ToolCallID == "" || e.Source != "native_tool_and_owner_verification" {
				if result != "failed" {
					result = "unverified"
				}
			}
		}
	}
	return result
}

// A caller can apply this guard without inventing a second digest algorithm.
func ValidateCheckUpdate(prior, next EnvironmentCheck) *Fault {
	conflict := prior.CheckID != next.CheckID || prior.InputDigest != next.InputDigest || prior.ResourceDigest != next.ResourceDigest || prior.Contract != next.Contract || !prior.CreatedAt.Equal(next.CreatedAt) || !prior.ExecutionDeadline.Equal(next.ExecutionDeadline) || !prior.CleanupDeadline.Equal(next.CleanupDeadline) || !reflect.DeepEqual(prior.Input, next.Input) || next.Revision < prior.Revision || next.Revision == prior.Revision && !reflect.DeepEqual(prior, next)
	oldContext, newContext := reflect.ValueOf(prior.Actual), reflect.ValueOf(next.Actual)
	for i := 0; i < oldContext.NumField(); i++ {
		if !oldContext.Field(i).IsZero() && !reflect.DeepEqual(oldContext.Field(i).Interface(), newContext.Field(i).Interface()) {
			conflict = true
		}
	}
	for _, pair := range [][2]string{{prior.Resources.BaseRef, next.Resources.BaseRef}, {prior.Resources.HeadRef, next.Resources.HeadRef}, {prior.Resources.BaseSHA, next.Resources.BaseSHA}, {prior.Resources.HeadSHA, next.Resources.HeadSHA}, {prior.Resources.PRURL, next.Resources.PRURL}} {
		if pair[0] != "" && pair[0] != pair[1] {
			conflict = true
		}
	}
	if prior.Resources.PRNumber != 0 && prior.Resources.PRNumber != next.Resources.PRNumber {
		conflict = true
	}
	if conflict {
		return Fail(5, "stale_revision", "Check identity, immutable context or revision conflicts with the saved snapshot.")
	}
	return nil
}

func DecodeCheckInput(raw json.RawMessage) CheckInput {
	var value CheckInput
	_ = json.Unmarshal(raw, &value)
	return value
}
