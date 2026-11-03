package orcatransport

import (
	"bytes"
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
)

// Decode only the current local and native remote profile contracts.
func decodeLifecycleProfile(setup b.LifecycleSetup) (Profile, int, error) {
	if setup.Snapshot.Ref.Backend != b.BackendOrca || setup.Snapshot.Ref.Protocol != b.ProtocolOrcaTerminalV3 {
		return Profile{}, 0, errors.New("unsupported Orca terminal lifecycle")
	}
	var p Profile
	version := 0
	found := false
	for _, data := range setup.ProfileData {
		if data.ImplementationKey != ProfileKey {
			continue
		}
		if found || data.CodecVersion != NativeRemoteProfileVersion && data.CodecVersion != FullAccessTerminalProfileVersion && data.CodecVersion != NativeProfileVersion {
			return Profile{}, 0, errors.New("invalid terminal profile codec")
		}
		found = true
		version = data.CodecVersion
		if err := data.ValidateProfileIdentity(setup.Snapshot.Session.ProjectID, setup.ProfileID, setup.AcceptedConfigVersion); err != nil {
			return Profile{}, 0, err
		}
		var err error
		p, err = decodeProfileCodec(data.Payload, version)
		if err != nil {
			return Profile{}, 0, err
		}
	}
	if !found || setup.Snapshot.Ref.Target.ID != p.Target().ID {
		return Profile{}, 0, errors.New("terminal profile differs from frozen target")
	}
	if p.NativeTerminal && setup.Snapshot.Ref.Agent.Name != "codex" {
		return Profile{}, 0, errors.New("native terminal profile requires Codex")
	}
	return p, version, nil
}

func decodeProfileCodec(raw json.RawMessage, version int) (Profile, error) {
	p, err := decodeProfileBinding(raw, version)
	if err != nil {
		return Profile{}, err
	}
	if version == FullAccessTerminalProfileVersion && (p.Environment != "" || p.Terminal != nil && (p.Terminal.ReceiptVersion != 4 || p.Terminal.AccessMode != "full_access" || len(p.Terminal.BundleSHA256) != 64 || len(p.Terminal.EvidenceSHA256) != 64)) {
		return Profile{}, errors.New("new full-access terminal profile is incomplete or contains retired settings")
	}
	if version == NativeRemoteProfileVersion {
		if err := rejectNativeHelperFields(raw); err != nil {
			return Profile{}, err
		}
		if p.Environment == "" {
			return Profile{}, errors.New("native remote profile has incompatible local/helper configuration")
		}
	}
	return p, nil
}

// Binding readers retain the historical profile contract. Launch preparation
// requirements must not retire already committed usage evidence. The new native
// codec still requires its complete, isolated native contract.
func decodeProfileBinding(raw json.RawMessage, version int) (Profile, error) {
	if version != NativeProfileVersion && version != NativeRemoteProfileVersion && version != FullAccessTerminalProfileVersion {
		return Profile{}, errors.New("invalid terminal profile codec")
	}
	p, err := DecodeProfile(raw)
	if err != nil {
		return Profile{}, err
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for key := range fields {
		if strings.EqualFold(key, "native_terminal") && (version != NativeProfileVersion || key != "native_terminal") {
			return Profile{}, errors.New("native terminal contract requires its original profile codec and canonical marker")
		}
	}
	if version == NativeProfileVersion && (!p.NativeTerminal || !bytes.Equal(bytes.TrimSpace(fields["native_terminal"]), []byte("true"))) {
		return Profile{}, errors.New("native terminal profile contract missing")
	}
	if version == NativeProfileVersion {
		if err := rejectNativeHelperFields(raw); err != nil {
			return Profile{}, err
		}
	}
	return p, nil
}

func rejectNativeHelperFields(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key := range fields {
		for _, helper := range []string{"terminal", "runtime_executable", "runtime_sha256", "runtime_configuration_digest"} {
			if strings.EqualFold(key, helper) {
				return errors.New("native terminal profile contains a helper field")
			}
		}
	}
	return nil
}
