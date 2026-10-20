package orcatransport

import (
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// ValidateRecovery shares the constructor's exact profile qualification, without
// constructing a lifecycle or giving it native process/transport capabilities.
func ValidateRecovery(setup b.LifecycleSetup) error {
	_, _, err := decodeLifecycleProfile(setup)
	return err
}
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
		if found || data.CodecVersion != ProfileVersion && data.CodecVersion != NativeRemoteProfileVersion {
			return Profile{}, 0, errors.New("invalid terminal profile codec")
		}
		found = true
		version = data.CodecVersion
		if err := data.ValidateProfileIdentity(setup.Snapshot.Session.ProjectID, setup.ProfileID, setup.AcceptedConfigVersion); err != nil {
			return Profile{}, 0, err
		}
		var err error
		p, err = DecodeProfile(data.Payload)
		if err != nil {
			return Profile{}, 0, err
		}
		if version == NativeRemoteProfileVersion {
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(data.Payload, &fields)
			for _, key := range []string{"terminal", "runtime_executable", "runtime_sha256", "runtime_configuration_digest"} {
				if _, present := fields[key]; present {
					return Profile{}, 0, errors.New("native remote profile contains a helper field")
				}
			}
		}
	}
	if !found || setup.Snapshot.Ref.Target.ID != p.Target().ID {
		return Profile{}, 0, errors.New("terminal profile differs from frozen target")
	}
	if version == NativeRemoteProfileVersion && (p.Environment == "" || p.Terminal != nil || p.RuntimeExecutable != "" || p.RuntimeSHA256 != "" || p.RuntimeConfigurationDigest != "") {
		return Profile{}, 0, errors.New("native remote profile has incompatible local/helper configuration")
	}
	return p, version, nil
}
