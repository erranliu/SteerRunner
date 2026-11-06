package orcatransport

import (
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

type TerminalConfig struct {
	ReceiptVersion      int    `json:"receipt_version,omitempty"`
	Agent               string `json:"agent"`
	BundleSHA256        string `json:"bundle_sha256,omitempty"`
	EvidenceSHA256      string `json:"evidence_sha256,omitempty"`
	ProjectID           string `json:"project_id"`
	ExecutionID         string `json:"execution_id"`
	AttemptID           string `json:"attempt_id"`
	Generation          string `json:"generation"`
	ConfigurationDigest string `json:"configuration_digest"`
	SessionID           string `json:"session_id"`
	Executable          string `json:"executable"`
	StateDirectory      string `json:"state_directory"`
	RepositoryURL       string `json:"repository_url"`
	AccessMode          string `json:"access_mode"`
	Model               string `json:"model,omitempty"`
	ReasoningEffort     string `json:"reasoning_effort,omitempty"`
}

func (c *TerminalConfig) UnmarshalJSON(raw []byte) error {
	type wire TerminalConfig
	var value wire
	if err := b.DecodeStrictJSON(raw, &value, "trailing terminal configuration"); err != nil {
		return err
	}
	*c = TerminalConfig(value)
	return nil
}

type TerminalOwner struct {
	Job                 process.OwnedJobReference `json:"job"`
	RuntimeSHA256       string                    `json:"runtime_sha256"`
	ExtensionSHA256     string                    `json:"extension_bundle_sha256"`
	SessionID           string                    `json:"session_id"`
	RunID               string                    `json:"run_id"`
	ConfigurationDigest string                    `json:"configuration_digest"`
	AccessMode          string                    `json:"access_mode"`
}
