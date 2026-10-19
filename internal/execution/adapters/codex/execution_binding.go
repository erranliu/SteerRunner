package codexworker

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const ExecutionBindingCodecKey = "codex.app-server-binding"

type AppServerBinding struct {
	Owner      *ExecutionOwnerIdentity `json:"owner,omitempty"`
	Endpoint   string                  `json:"endpoint"`
	SessionID  string                  `json:"session_id,omitempty"`
	Generation string                  `json:"generation"`
}

type AppServerBindingIdentity struct {
	ProjectID    string
	ExecutionID  string
	AttemptID    string
	Generation   string
	ConfigDigest string
}

func NewAppServerBindingEnvelope(identity AppServerBindingIdentity, binding AppServerBinding) (executionbackend.ImplementationDataEnvelope, error) {
	if identity.Generation == "" || binding.SessionID != "" && !ValidNativeID(binding.SessionID) ||
		binding.Endpoint == "" && (binding.SessionID == "" || binding.Generation != "") ||
		binding.Endpoint != "" && binding.Generation == "" {
		return executionbackend.ImplementationDataEnvelope{}, errors.New("app-server binding is incomplete")
	}
	if binding.Endpoint != "" {
		if err := codexapp.ValidateEndpoint(binding.Endpoint); err != nil {
			return executionbackend.ImplementationDataEnvelope{}, err
		}
	}
	payload, err := json.Marshal(binding)
	if err != nil {
		return executionbackend.ImplementationDataEnvelope{}, err
	}
	version := 1
	if binding.Owner != nil {
		version = 2
	}
	return executionbackend.NewImplementationDataEnvelope(executionbackend.ImplementationDataEnvelope{
		CodecVersion: version, Purpose: executionbackend.ImplementationResourcePurpose,
		ImplementationKey: ExecutionBindingCodecKey, ProjectID: identity.ProjectID,
		ExecutionID: identity.ExecutionID, AttemptID: identity.AttemptID,
		Generation: identity.Generation, ConfigDigest: identity.ConfigDigest,
	}, payload)
}

func DecodeAppServerBindingEnvelope(envelope executionbackend.ImplementationDataEnvelope, identity AppServerBindingIdentity) (AppServerBinding, error) {
	if envelope.ImplementationKey != ExecutionBindingCodecKey || (envelope.CodecVersion != 1 && envelope.CodecVersion != 2) {
		return AppServerBinding{}, errors.New("unsupported app-server binding codec")
	}
	if err := envelope.ValidateResourceIdentity(identity.ProjectID, identity.ExecutionID, identity.AttemptID, identity.Generation, identity.ConfigDigest); err != nil {
		return AppServerBinding{}, err
	}
	var binding AppServerBinding
	if err := executionbackend.DecodeStrictJSON(envelope.Payload, &binding, "app-server binding payload has trailing JSON"); err != nil {
		return AppServerBinding{}, err
	}
	if (envelope.CodecVersion == 2) != (binding.Owner != nil) {
		return AppServerBinding{}, errors.New("owner binding codec does not match resource ownership")
	}
	if owner := binding.Owner; owner != nil && (owner.Protocol != ExecutionOwnerProtocol || !filepath.IsAbs(owner.Directory) || owner.Process.Kind != "os" || owner.Process.PID <= 0 || owner.Process.Created == "" || owner.Process.Executable == "" || len(owner.RuntimeSHA256) != 64 || len(owner.ExitPublicKey) != 64) {
		return AppServerBinding{}, errors.New("execution owner identity is incomplete")
	}
	if binding.SessionID != "" && !ValidNativeID(binding.SessionID) ||
		binding.Endpoint == "" && (binding.SessionID == "" || binding.Generation != "") ||
		binding.Endpoint != "" && binding.Generation == "" {
		return AppServerBinding{}, errors.New("app-server binding differs from its fixed native identity")
	}
	if binding.Endpoint != "" {
		if err := codexapp.ValidateEndpoint(binding.Endpoint); err != nil {
			return AppServerBinding{}, err
		}
	}
	return binding, nil
}
