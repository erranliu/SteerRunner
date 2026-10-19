package livetest

import (
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Bytes uses JSON's base64 encoding. Embedding the document as RawMessage lets
// an outer MarshalIndent rewrite its signed body and invalidate the evidence.
type SignedNativeExitEvidence struct {
	Proof   codex.RemoteAuthenticationCleanup `json:"proof"`
	Control struct {
		Process *b.ProcessIdentity `json:"process"`
		Closed  bool               `json:"closed"`
	} `json:"control"`
	Bytes []byte `json:"signed_native_exit_bytes"`
}
