package livetest

import (
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
)

// Bytes uses JSON's base64 encoding. Embedding the document as RawMessage lets
// an outer MarshalIndent rewrite its signed body and invalidate the evidence.
type SignedNativeExitEvidence struct {
	Proof   codex.RemoteAuthenticationCleanup `json:"proof"`
	Control orcatransport.ControlClosure      `json:"control"`
	Bytes   []byte                            `json:"signed_native_exit_bytes"`
}
