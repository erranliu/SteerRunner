package application

import (
	"fmt"
	"slices"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// TakeoverFacts projects existing records through the selected build's concrete
// decoders. It neither constructs workers nor grants any input authorization.
func TakeoverFacts(runs []*runstate.Record, checks []*runstate.CheckRecord, support contracts.RecoverySupport, profiles []workflow.Profile) []contracts.TakeoverItem {
	items := []contracts.TakeoverItem{}
	for _, r := range runs {
		item := contracts.TakeoverItem{ID: r.ID, Kind: "run", Revision: r.Revision, InputDigest: r.InputDigest, Status: "compatible"}
		var err error
		if e := r.AcquisitionConfig.Config.Validate(); e != nil {
			err = e
		}
		if e := verifyFrozenMaterials(r); e != nil {
			err = e
		}
		if !slices.Contains(support.ExecutionRecordVersions, r.Snapshot.Ref.Version) {
			item.Status, item.Diagnostic = "unsupported", "execution record version unsupported"
			items = append(items, item)
			continue
		}
		if orca.LegacyRemoteProfile(r.Profile) {
			item.Status = "read_only"
			item.Diagnostic = "retired remote helper execution remains read-only"
			items = append(items, item)
			continue
		}
		found := false
		for _, p := range profiles {
			if p.ID == r.Request.Workflow.ID && p.Version == r.Request.Workflow.Version {
				found = true
				if r.Workflow.Config == nil || !compatibleWorkflow(p, *r.Workflow.Config, r.Request.Execution.Adapter) {
					err = fmt.Errorf("frozen workflow execution semantics are unsupported")
				}
			}
		}
		if !found {
			err = fmt.Errorf("workflow interpreter unavailable")
		}
		for _, intent := range r.Intents {
			if !slices.Contains(support.OperationVersions, intent.Request.Operation.Version) {
				err = fmt.Errorf("operation version unsupported")
			}
			if p := intent.Request.Input.Parse; p != nil {
				var pack parser.Package
				if intent.Request.Validate() != nil || b.DecodeStrictJSON([]byte(p.Payload), &pack, "invalid frozen parser package") != nil || pack.Profile != parser.Profile || pack.Schema != string(ra.ParserSchema()) || pack.Config.Config.Validate() != nil {
					err = fmt.Errorf("frozen parser/result acquisition contract unsupported")
				}
			}
		}
		for _, op := range r.Snapshot.Operations {
			if !slices.Contains(support.OperationVersions, op.Version) {
				err = fmt.Errorf("snapshot operation version unsupported")
			}
		}
		for _, envelope := range append(append([]b.ImplementationDataEnvelope{}, r.Profile...), r.Snapshot.Implementation...) {
			if !slices.Contains(support.ImplementationCodecs[envelope.ImplementationKey], envelope.CodecVersion) {
				err = fmt.Errorf("implementation %s codec %d unsupported", envelope.ImplementationKey, envelope.CodecVersion)
			}
		}
		if r.Request.Execution.Adapter == "codex-app-server" {
			if _, e := codex.DecodeProfileConfigEnvelope(r.Profile, codex.ProfileConfigIdentity{ProjectID: r.Snapshot.Session.ProjectID, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}); e != nil {
				err = e
			}
			for _, envelope := range r.Snapshot.Implementation {
				if envelope.ImplementationKey == codex.ExecutionBindingCodecKey {
					binding, e := codex.DecodeAppServerBindingEnvelope(envelope, codex.AppServerBindingIdentity{ProjectID: r.Snapshot.Session.ProjectID, ExecutionID: r.Snapshot.Session.ExecutionID, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.ResourceGeneration, ConfigDigest: r.Snapshot.ConfigurationDigest})
					if e != nil {
						err = e
					} else if binding.Owner == nil && r.StopStatus != "stopped" && process.InspectProcess(&r.Snapshot.Ref.External) != "process absent" {
						err = fmt.Errorf("legacy service owns the live native Job; independent transfer is unavailable")
					} else if binding.Owner != nil && binding.Owner.Protocol != support.OwnerProtocol {
						err = fmt.Errorf("original execution owner protocol unsupported")
					} else if binding.Owner != nil && r.StopStatus != "stopped" && r.Status != "pr_ready" && ((!binding.Owner.RetainsParser && r.AcquisitionConfig.Config.CodexParser.Enabled) || !binding.Owner.WorkspaceCheckpoints && r.Workflow.RequiresWorkspace()) {
						err = fmt.Errorf("original owner lacks required parser/completion-checkpoint capability; no live owner replaced")
					}
				}
			}
			if r.Snapshot.PolicyResources != nil {
				p := r.Snapshot.PolicyResources
				if !slices.Contains(support.ImplementationCodecs[p.ImplementationKey], p.CodecVersion) {
					err = fmt.Errorf("permission resource codec unsupported")
				}
				s := r.Snapshot
				if _, e := codex.DecodePolicyResourceEnvelope(*p, codex.PolicyResourceIdentity{ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}); e != nil {
					err = e
				}
			}
		} else if r.Request.Execution.Adapter == "orca-terminal" {
			_, e := orca.NewLifecycle(b.LifecycleSetup{Snapshot: r.Snapshot, ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest, ProcessHost: process.New(0)})
			if e != nil {
				err = e
			}
		} else {
			err = fmt.Errorf("execution adapter unsupported")
		}
		if err != nil {
			item.Status = "unsupported"
			item.Diagnostic = err.Error()
		}
		items = append(items, item)
	}
	for _, r := range checks {
		item := contracts.TakeoverItem{ID: r.View.CheckID, Kind: "check", Revision: r.View.Revision, InputDigest: r.View.InputDigest, Status: "compatible"}
		if r.View.Input.Execution.Adapter == "orca-terminal" && r.View.Input.Execution.Target.Kind != "local" {
			item.Status = "read_only"
			item.Diagnostic = "remote Check actors are unsupported; original facts remain queryable"
		}
		items = append(items, item)
	}
	return items
}

func (e *Engine) Takeover(support contracts.RecoverySupport, profiles []workflow.Profile) []contracts.TakeoverItem {
	e.mu.Lock()
	runs := make([]*runstate.Record, 0, len(e.runs))
	for _, r := range e.runs {
		runs = append(runs, r.Clone())
	}
	checks := make([]*runstate.CheckRecord, 0, len(e.checks))
	for _, r := range e.checks {
		checks = append(checks, r.Clone())
	}
	e.mu.Unlock()
	return e.store.BindTakeoverRequests(TakeoverFacts(runs, checks, support, profiles))
}
