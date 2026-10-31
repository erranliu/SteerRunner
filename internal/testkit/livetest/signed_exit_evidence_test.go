package livetest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"testing"
	"time"
)

func TestSignedNativeExitSurvivesIndentedArchive(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ready := codex.RemoteReadiness{ExitPublicKey: public, CheckedAt: time.Now().UTC()}
	body, _ := json.Marshal(codex.RemoteExit{Readiness: ready, WritersAbsent: true, ClosedAt: time.Now().UTC()})
	signed, _ := json.Marshal(struct {
		Body      json.RawMessage `json:"body"`
		Signature []byte          `json:"signature"`
	}{body, ed25519.Sign(private, body)})
	if err = codex.VerifyRemoteExit(signed, ready); err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.MarshalIndent(map[string]any{"exit": json.RawMessage(signed)}, "", "  ")
	var lossy struct {
		Exit json.RawMessage `json:"exit"`
	}
	_ = json.Unmarshal(legacy, &lossy)
	if codex.VerifyRemoteExit(lossy.Exit, ready) == nil {
		t.Fatal("fixture did not reproduce signature-body reformatting")
	}
	archived, err := json.MarshalIndent(SignedNativeExitEvidence{Bytes: signed}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var recovered SignedNativeExitEvidence
	if err = json.Unmarshal(archived, &recovered); err != nil || !bytes.Equal(signed, recovered.Bytes) {
		t.Fatal("archive changed original signed bytes", err)
	}
	if err = codex.VerifyRemoteExit(recovered.Bytes, ready); err != nil {
		t.Fatal("archive could not be verified", err)
	}
	recovered.Bytes[len(recovered.Bytes)/2] ^= 1
	if codex.VerifyRemoteExit(recovered.Bytes, ready) == nil {
		t.Fatal("altered archived proof was accepted")
	}
}
