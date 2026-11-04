package contracts

import "encoding/base64"

const MaxResultBytes = MaxProgressBytes
const ResultFileContract = "steerrunner-result-file/v1"
const ResultFileEnvironment = "STEERLINE_RESULT_FILE"

type ResultFileInput struct {
	InitialContentBase64 string `json:"initial_content_base64"`
	RequiredInput        bool   `json:"required_input,omitempty"`
}

func (f *ResultFileInput) IsRequiredInput() bool {
	return f != nil && f.RequiredInput
}

type ResultFileView = ProgressFileView

func ValidateResultFile(r RunRequest) *Fault {
	if r.ResultFile == nil {
		return nil
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(r.ResultFile.InitialContentBase64)
	if err != nil || len(raw) > MaxResultBytes || base64.StdEncoding.EncodeToString(raw) != r.ResultFile.InitialContentBase64 {
		return Fail(2, "invalid_input", "result_file requires canonical base64 of at most 65536 raw bytes.")
	}
	if r.ResultFile.RequiredInput && len(raw) == 0 {
		return Fail(2, "invalid_input", "A required input file must have nonempty original bytes.")
	}
	if (r.Execution.Adapter != "codex-app-server" && r.Execution.Adapter != "orca-terminal") || r.Conversation != nil || r.Generate != nil {
		return Fail(3, "unsupported_capability", "Result files require a writable run workspace on local Codex or Orca.")
	}
	return nil
}
