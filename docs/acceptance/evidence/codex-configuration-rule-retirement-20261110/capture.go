// One-shot acceptance capture; not a product or test-framework entry point.
package main

import (
	"encoding/json"
	"os"
	"strings"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/resources"
)

func main() {
	var profiles []json.RawMessage
	var envelopes []b.ImplementationDataEnvelope
	for _, mode := range []string{"full_access", "read_only"} {
		for _, model := range []string{"", "explicit-model"} {
			p := codex.ProfileConfig{AccessMode: mode, Executable: "C:/fixed/codex.exe", Model: model, ReasoningEffort: "high"}
			raw, err := codex.EncodeProfileConfig(p)
			must(err)
			profiles = append(profiles, raw)
			e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: codex.ProfileVersionForMode(mode), Purpose: b.ImplementationConfigPurpose, ImplementationKey: codex.ProfileCodecKey, ProjectID: "project", ProfileID: "profile", AcceptedConfigVersion: "config"}, raw)
			must(err)
			envelopes = append(envelopes, e)
		}
	}
	out := struct {
		Profiles       []json.RawMessage
		Envelopes      []b.ImplementationDataEnvelope
		Capabilities   any
		Schemas        json.RawMessage
		ResourceDigest string
	}{profiles, envelopes, resources.CapabilitiesFrom(resources.CapabilitySource{Kind: "cli", Version: resources.Version, Protocol: "steerrunner-cli/v1", ExecutableSHA256: strings.Repeat("a", 64), ResourceDigest: resources.Digest()}), resources.ConfigurationSchemas(), resources.Digest()}
	raw, err := json.Marshal(out)
	must(err)
	must(os.WriteFile(os.Args[1], raw, 0600))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
