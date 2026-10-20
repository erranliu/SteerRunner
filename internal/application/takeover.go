package application

import (
	"fmt"
	"slices"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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
		// Content workflows have no result-acquisition/parser contract. Their
		// accepted records deliberately retain an empty acquisition config.
		if !workflow.Content(r.Request.Workflow.ID) {
			if e := r.AcquisitionConfig.Config.Validate(); e != nil {
				err = e
			}
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
				if parser.ValidateRecovery(intent.Request) != nil {
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

			setup := b.LifecycleSetup{Snapshot: r.Snapshot, ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}
			active := r.StopStatus != "stopped" && r.Status != "pr_ready"
			needs := b.RecoveryRequirements{Stopped: r.StopStatus == "stopped", Parser: active && r.AcquisitionConfig.Config.CodexParser.Enabled, WorkspaceCheckpoints: active && r.Workflow.RequiresWorkspace()}
			if e := codex.ValidateRecovery(setup, support.ImplementationCodecs, support.OwnerProtocol, needs); e != nil {
				err = e
			}
		} else if r.Request.Execution.Adapter == "orca-terminal" {
			e := orca.ValidateRecovery(b.LifecycleSetup{Snapshot: r.Snapshot, ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest})
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
		} else {
			// Checks retain raw execution settings rather than a Run profile
			// envelope. Qualify them too, before reporting that a new writer can
			// adopt the state. In particular retired App authentication must not
			// silently become the current host's gh identity.
			if err := contracts.ValidateExecution(r.View.Input.Execution); err != nil {
				item.Status, item.Diagnostic = "unsupported", "frozen Check execution configuration unsupported"
			}
			for _, op := range r.Operations {
				if !slices.Contains(support.OperationVersions, op.Version) {
					item.Status, item.Diagnostic = "unsupported", "Check operation version unsupported"
				}
			}
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
