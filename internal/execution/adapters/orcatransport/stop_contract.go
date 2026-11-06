package orcatransport

import (
	"slices"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// DevelopmentCloseContract classifies the saved contract marker, not its proof.
// Payload and identity validation remain at the original execution/read boundary.
// The caller still owns the Workflow and command admission rules.
func DevelopmentCloseContract(protocol b.ExecutionProtocol, profiles []b.ImplementationDataEnvelope) bool {
	return protocol == b.ProtocolOrcaAgentSessionV1 && slices.ContainsFunc(profiles, func(p b.ImplementationDataEnvelope) bool {
		return p.ImplementationKey == AgentSessionProfileKey && p.CodecVersion == AgentSessionCloseProfileVersion
	})
}

// RequiresFrozenStopScope reports the backend's original scope requirement.
// An already saved scope must also be preserved, independently of this marker.
func RequiresFrozenStopScope(profiles []b.ImplementationDataEnvelope, scope string) bool {
	return scope == LocalCodexStopScope || slices.ContainsFunc(profiles, func(p b.ImplementationDataEnvelope) bool {
		return p.ImplementationKey == ProfileKey && p.CodecVersion == NativeProfileVersion
	})
}

// ConfirmedStopDiagnostic describes a completed stop using its saved scope.
// The caller retains the original stopped/Orca-port/nonempty-port-scope guards.
func ConfirmedStopDiagnostic(scope string) string {
	switch scope {
	case LocalCodexStopScope:
		return LocalCodexStopDiagnostic
	case AgentSessionStopScope:
		return AgentSessionStopLimitation
	default:
		return NativeStopDiagnostic
	}
}
