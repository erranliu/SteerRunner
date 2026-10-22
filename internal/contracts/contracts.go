// Package contracts owns the public CLI values, not storage or native bindings.
package contracts

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/erranliu/SteerRunner/internal/usage/api"
)

const Protocol = "steerrunner-cli/v1"
const MaxRequestBytes = 8 * 1024 * 1024

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Fault struct {
	Detail Error
	Exit   int
}

func (f *Fault) Error() string { return f.Detail.Message }

func Fail(exit int, code, message string) *Fault {
	return &Fault{Detail: Error{Code: code, Message: message}, Exit: exit}
}

type Request struct {
	Protocol  string          `json:"protocol"`
	RequestID string          `json:"request_id"`
	Input     json.RawMessage `json:"input"`
}

type Response struct {
	Protocol  string  `json:"protocol"`
	OK        bool    `json:"ok"`
	RequestID *string `json:"request_id"`
	Data      any     `json:"data"`
	Error     *Error  `json:"error"`
}

func Success(id string, data any) Response {
	var requestID *string
	if id != "" {
		requestID = &id
	}
	return Response{Protocol: Protocol, OK: true, RequestID: requestID, Data: data}
}

func Rejected(id string, fault *Fault) Response {
	response := Success(id, nil)
	response.OK, response.Error = false, &fault.Detail
	return response
}

func ExitCode(response Response) int {
	if response.Error != nil {
		switch response.Error.Code {
		case "session_closed":
			return 5
		case "invalid_output_schema":
			return 2
		case "directory_unavailable":
			return 4
		}
	}
	if response.OK {
		return 0
	}
	if response.Error == nil {
		return 1
	}
	switch response.Error.Code {
	case "artifact_path_invalid", "artifact_limit_exceeded":
		return 2
	case "workspace_unavailable", "artifact_unavailable":
		return 4
	case "workspace_source_unknown", "artifact_not_found", "artifact_changed", "artifact_integrity_failed":
		return 5
	case "invalid_arguments", "invalid_request", "invalid_input", "invalid_cursor":
		return 2
	case "unsupported_protocol", "unsupported_schema", "unsupported_capability", "unsupported_platform":
		return 3
	case "service_unavailable", "service_start_failed", "detach_unsupported", "startup_unverified", "environment_unavailable":
		return 4
	case "snapshot_not_found", "snapshot_expired", "invalid_usage_cursor", "usage_policy_conflict", "usage_observation_stale", "request_conflict", "request_not_found", "command_not_found", "run_not_found", "check_not_found", "stale_instance", "directory_conflict", "stale_revision", "invalid_state", "stale_question", "resume_required", "operation_unverified", "answer_pending", "repository_conflict":
		return 5
	case "request_outcome_unknown":
		return 6
	default:
		return 1
	}
}

func ReadRequest(reader io.Reader) (Request, *Fault) {
	var request Request
	data, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if err != nil || len(data) > MaxRequestBytes || DecodeStrict(data, &request) != nil {
		return request, Fail(2, "invalid_request", "Expected one bounded UTF-8 JSON request with unique, recognized fields.")
	}
	if request.Protocol == "" || !ValidRequestID(request.RequestID) || len(request.Input) == 0 || string(request.Input) == "null" {
		return request, Fail(2, "invalid_request", "protocol, a bounded request_id and an input object are required.")
	}
	if request.Protocol != Protocol {
		return request, Fail(3, "unsupported_protocol", "This executable supports "+Protocol+".")
	}
	return request, nil
}

func ValidRequestID(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 256 && strings.IndexFunc(value, unicode.IsControl) < 0
}

type ServiceInfo struct {
	InstanceID          string           `json:"instance_id"`
	StateDir            string           `json:"state_dir"`
	PID                 int              `json:"pid"`
	ProcessCreated      string           `json:"process_created"`
	Status              string           `json:"status"`
	Detached            bool             `json:"detached"`
	Version             string           `json:"version"`
	Protocol            string           `json:"protocol"`
	ResourceDigest      string           `json:"resource_digest"`
	ExecutableSHA256    string           `json:"executable_sha256"`
	LastShutdown        *ServiceShutdown `json:"last_shutdown,omitempty"`
	UsagePolicyDigest   string           `json:"usage_policy_digest,omitempty"`
	UsagePolicyRevision string           `json:"usage_policy_revision,omitempty"`
	UsagePolicy         *api.Policy      `json:"usage_policy,omitempty"`
}

// ServiceShutdown is published after worker shutdown, before releasing the
// writer lease. Service exit by itself is not native resource evidence.
type ServiceShutdown struct {
	InstanceID       string `json:"instance_id"`
	RequestID        string `json:"request_id"`
	StateDir         string `json:"state_dir"`
	ExecutableSHA256 string `json:"executable_sha256"`
	ResourcesSettled bool   `json:"resources_settled"`
	Diagnostic       string `json:"diagnostic,omitempty"`
}

type ServiceStopInput struct {
	ExpectedInstanceID string `json:"expected_instance_id"`
}

type StoppedVerification struct {
	Checks           []StoppedCheck `json:"checks"`
	StateDir         string         `json:"state_dir"`
	ResourcesSettled bool           `json:"resources_settled"`
	Runs             []StoppedRun   `json:"runs"`
}

type StoppedCheck struct {
	CheckID          string `json:"check_id"`
	InputDigest      string `json:"input_digest"`
	ResourcesSettled bool   `json:"resources_settled"`
}

type StoppedRun struct {
	StopEvidenceScope string `json:"stop_evidence_scope,omitempty"`
	RunID             string `json:"run_id"`
	InputDigest       string `json:"input_digest"`
	ResourcesSettled  bool   `json:"resources_settled"`
	Diagnostic        string `json:"diagnostic,omitempty"`
}

type CommandView struct {
	RequestID     string  `json:"request_id"`
	CommandID     string  `json:"command_id"`
	Command       string  `json:"command"`
	RunID         *string `json:"run_id"`
	ReceiptStatus string  `json:"receipt_status"`
	ActionStatus  string  `json:"action_status"`
	Error         *Error  `json:"error"`
}

func NewID() string {
	// Set by the sole ID implementation, shared by host and persistence.
	return randomID()
}

func Encode(response Response, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}
