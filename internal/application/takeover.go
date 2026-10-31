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
func TakeoverFacts(runs []*runstate.Record, support contracts.RecoverySupport, profiles []workflow.Profile, resultContracts ...string) []contracts.TakeoverItem {
	items := []contracts.TakeoverItem{}
	for _, r := range runs {
		item := contracts.TakeoverItem{ID: r.ID, Kind: "run", Revision: r.Revision, InputDigest: r.InputDigest, Status: "compatible"}
		if r.Request.ResultFile != nil && (len(resultContracts) != 1 || resultContracts[0] != contracts.ResultFileContract) {
			item.Status, item.Diagnostic = "unsupported", "result file contract unsupported"
			items = append(items, item)
			continue
		}
		var err error
		requiresAcquisition := r.Workflow.Config != nil && r.Workflow.Config.RequiresResultAcquisition()
		hasParser := false
		for _, intent := range r.Intents {
			hasParser = hasParser || intent.Request.Input.Parse != nil
		}
		if requiresAcquisition || hasParser {
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
		if readOnlyRun(r) {
			item.Status = readOnlyStatus(r)
			item.Diagnostic = readOnlyProjection(r).Diagnostic
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
			if q := intent.PreparationQuery; q != nil {
				if e := q.Validate(intent.Request.Operation); e != nil {
					err = e
				}
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
			needs := b.RecoveryRequirements{Stopped: r.StopStatus == "stopped", Parser: active && (hasParser || requiresAcquisition && r.AcquisitionConfig.Config.CodexParser.Enabled), WorkspaceCheckpoints: active && r.Workflow.RequiresWorkspace()}
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
	return items
}

func (e *Engine) Takeover(support contracts.RecoverySupport, profiles []workflow.Profile, resultContracts ...string) []contracts.TakeoverItem {
	e.mu.Lock()
	runs := make([]*runstate.Record, 0, len(e.runs))
	for _, r := range e.runs {
		runs = append(runs, r.Clone())
	}
	e.mu.Unlock()
	return e.store.BindTakeoverRequests(TakeoverFacts(runs, support, profiles, resultContracts...))
}
