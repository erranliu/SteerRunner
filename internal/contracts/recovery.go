package contracts

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

type PendingStop struct {
	RunID   string  `json:"run_id"`
	Request Request `json:"request"`
}
type ServiceStartInput struct {
	PendingStops []PendingStop `json:"pending_stops"`
	UsagePolicy  *api.Policy   `json:"usage_policy,omitempty"`
}

func ReadServiceStartInput(raw []byte) (*ServiceStartInput, *Fault) {
	var input ServiceStartInput
	if len(raw) > MaxRequestBytes || DecodeStrict(raw, &input) != nil || input.PendingStops == nil || len(input.PendingStops) > 500 {
		return nil, Fail(2, "invalid_request", "service start requires a pending_stops array of original stop requests.")
	}
	seen := map[string]bool{}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if policy, ok := fields["usage_policy"]; ok {
		if string(policy) == "null" || input.UsagePolicy == nil || input.UsagePolicy.Validate() != nil {
			return nil, Fail(2, "invalid_request", "Invalid usage_policy.")
		}
		var members map[string]json.RawMessage
		_ = json.Unmarshal(policy, &members)
		if members["enabled"] == nil || string(members["enabled"]) == "null" || members["remote_log_access"] == nil {
			return nil, Fail(2, "invalid_request", "usage_policy requires enabled and remote_log_access.")
		}
	}
	for _, stop := range input.PendingStops {
		if !ValidRequestID(stop.RunID) || !ValidRequestID(stop.Request.RequestID) || stop.Request.Protocol != Protocol || seen[stop.Request.RequestID] {
			return nil, Fail(2, "invalid_request", "Invalid or duplicate pending stop identity.")
		}
		seen[stop.Request.RequestID] = true
		var revision RevisionInput
		if DecodeStrict(stop.Request.Input, &revision) != nil || revision.ExpectedRevision == 0 {
			return nil, Fail(2, "invalid_request", "Pending stops must preserve their original expected_revision.")
		}
	}
	return &input, nil
}

// RecoverySupport describes decoders, not a second per-Run contract manifest.
type RecoverySupport struct {
	ReadableStateFormats    []string         `json:"readable_state_formats"`
	WritableStateFormat     string           `json:"writable_state_format"`
	OperationVersions       []int            `json:"operation_versions"`
	ExecutionRecordVersions []int            `json:"execution_record_versions"`
	ImplementationCodecs    map[string][]int `json:"implementation_codecs"`
	OwnerProtocol           string           `json:"owner_protocol"`
	PendingStops            bool             `json:"pending_stops"`
}
type TakeoverItem struct {
	RequestID     string `json:"request_id,omitempty"`
	RequestDigest string `json:"request_digest,omitempty"`
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Revision      uint64 `json:"revision"`
	InputDigest   string `json:"input_digest"`
	Status        string `json:"status"`
	Diagnostic    string `json:"diagnostic,omitempty"`
}
type TakeoverReport struct {
	Contract        string         `json:"contract"`
	StateDir        string         `json:"state_dir"`
	InstanceID      string         `json:"instance_id,omitempty"`
	CandidateSHA256 string         `json:"candidate_sha256"`
	Compatible      bool           `json:"compatible"`
	Items           []TakeoverItem `json:"items"`
}

func (v ServiceStartInput) JSON() []byte { b, _ := json.Marshal(v); return b }
