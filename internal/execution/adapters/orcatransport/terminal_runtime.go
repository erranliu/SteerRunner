package orcatransport

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"

	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"

	"os"

	"strings"
)

//go:embed extensions/pi-evidence.mjs extensions/pi-managed.mjs extensions/terminal-tools.mjs extensions/receipt.mjs
var terminalAssets embed.FS

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

type TerminalMetadata struct {
	// Existing Pi helpers also emitted this shared metadata key. Accept the
	// unused value so removing DSH does not reject a frozen Pi helper build.
	DSHSHA256      string `json:"dsh_sha256,omitempty"`
	Authentication string `json:"authentication"`
	CodexTransport string `json:"codex_transport"`
	CodexPolicy    string `json:"codex_policy"`
	Version        int    `json:"version"`
	Contract       string `json:"contract"`
	RuntimeSHA256  string `json:"runtime_sha256"`
	BundleSHA256   string `json:"bundle_sha256"`
	PiSHA256       string `json:"pi_sha256"`
}

// Metadata is captured from the selected helper before accepting a Run, so an
// updated Runner can still read the original, frozen helper's evidence.
func TerminalRuntimeMetadata() (TerminalMetadata, error) {
	self, e := os.Executable()
	if e != nil {
		return TerminalMetadata{}, e
	}
	raw, e := os.ReadFile(self)
	if e != nil {
		return TerminalMetadata{}, e
	}
	pi, _ := terminalAssets.ReadFile("extensions/pi-evidence.mjs")
	return TerminalMetadata{Version: 5, Contract: terminaltools.Contract, Authentication: "host-native", CodexTransport: codex.RemoteNativeTransport, CodexPolicy: codex.PolicyContract, RuntimeSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), BundleSHA256: TerminalExtensionDigest(), PiSHA256: fmt.Sprintf("%x", sha256.Sum256(pi))}, nil
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

func (c TerminalConfig) Command(runtime string) string {
	raw, _ := json.Marshal(c)
	return "& '" + strings.ReplaceAll(runtime, "'", "''") + "' '--terminal-configuration' '" + base64.RawURLEncoding.EncodeToString(raw) + "'"
}

func TerminalExtensionDigest() string {
	h := sha256.New()
	for _, name := range []string{"pi-evidence.mjs", "pi-managed.mjs", "terminal-tools.mjs", "receipt.mjs"} {
		raw, _ := terminalAssets.ReadFile("extensions/" + name)
		h.Write([]byte(name))
		h.Write(raw)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
