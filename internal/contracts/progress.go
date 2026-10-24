package contracts

import (
	"encoding/base64"
	"time"
)

const MaxProgressBytes = 64 << 10
const ProgressContract = "steerrunner-progress-file/v1"
const ProgressEnvironment = "STEERLINE_PROGRESS_FILE"

type ProgressFileInput struct {
	InitialContentBase64 string `json:"initial_content_base64"`
}

// Content is optional until a complete snapshot has actually been received.
type ProgressFileView struct {
	Contract         string     `json:"contract"`
	RunID            string     `json:"run_id"`
	Version          uint64     `json:"version"`
	ObservedAt       *time.Time `json:"observed_at,omitempty"`
	Encoding         string     `json:"encoding"`
	ContentBase64    *string    `json:"content_base64,omitempty"`
	SizeBytes        int        `json:"size_bytes"`
	SHA256           string     `json:"sha256,omitempty"`
	CollectionStatus string     `json:"collection_status"`
	Diagnostic       string     `json:"diagnostic,omitempty"`
}

func ValidateProgress(r RunRequest) *Fault {
	if r.ProgressFile == nil {
		return nil
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(r.ProgressFile.InitialContentBase64)
	if err != nil || len(raw) > MaxProgressBytes || base64.StdEncoding.EncodeToString(raw) != r.ProgressFile.InitialContentBase64 {
		return Fail(2, "invalid_input", "progress_file requires canonical base64 of at most 65536 raw bytes.")
	}
	if (r.Execution.Adapter != "codex-app-server" && r.Execution.Adapter != "orca-terminal") || r.Conversation != nil || r.Generate != nil {
		return Fail(3, "unsupported_capability", "Progress files require a writable development workspace on local Codex or Orca.")
	}
	return nil
}
