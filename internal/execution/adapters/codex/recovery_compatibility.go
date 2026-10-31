package codexworker

import (
	"fmt"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"slices"
)

// ValidateRecovery decodes original implementation data and probes only the
// legacy root when required. It creates no lifecycle, native connection or files.
func ValidateRecovery(setup b.LifecycleSetup, codecs map[string][]int, ownerProtocol string, needs b.RecoveryRequirements) error {
	var err error
	if _, e := DecodeProfileConfigEnvelope(setup.ProfileData, ProfileConfigIdentity{ProjectID: setup.Snapshot.Session.ProjectID, ProfileID: setup.ProfileID, AcceptedConfigVersion: setup.AcceptedConfigVersion}); e != nil {
		err = e
	}
	for _, envelope := range setup.Snapshot.Implementation {
		if envelope.ImplementationKey == ExecutionBindingCodecKey {
			binding, e := DecodeAppServerBindingEnvelope(envelope, AppServerBindingIdentity{ProjectID: setup.Snapshot.Session.ProjectID, ExecutionID: setup.Snapshot.Session.ExecutionID, AttemptID: setup.Snapshot.Session.AttemptID, Generation: setup.Snapshot.ResourceGeneration, ConfigDigest: setup.Snapshot.ConfigurationDigest})
			if e != nil {
				err = e
			} else if binding.Owner == nil && !needs.Stopped && process.InspectProcess(&setup.Snapshot.Ref.External) != "process absent" {
				err = fmt.Errorf("legacy service owns the live native Job; independent transfer is unavailable")
			} else if binding.Owner != nil && binding.Owner.Protocol != ownerProtocol {
				err = fmt.Errorf("original execution owner protocol unsupported")
			} else if binding.Owner != nil && !needs.Stopped && (!binding.Owner.WorkspaceCheckpoints && needs.WorkspaceCheckpoints) {
				err = fmt.Errorf("original owner lacks required completion-checkpoint capability; no live owner replaced")
			}
		}
	}
	if setup.Snapshot.PolicyResources != nil {
		p := setup.Snapshot.PolicyResources
		if !slices.Contains(codecs[p.ImplementationKey], p.CodecVersion) {
			err = fmt.Errorf("permission resource codec unsupported")
		}
		s := setup.Snapshot
		if _, e := DecodePolicyResourceEnvelope(*p, PolicyResourceIdentity{ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}); e != nil {
			err = e
		}
	}
	return err
}
