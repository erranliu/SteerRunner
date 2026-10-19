package workflow

import (
	"errors"
)

// Legacy empty outcomes keep their existing interpretation. Explicit native
// non-success must not be overridden by a persisted coarse Succeeded flag.
func nativeOutcomeAllowsAcquisition(outcome string) bool {
	return outcome == "" || outcome == "succeeded" || outcome == "unreported"
}

func trustedDone(w Snapshot, id string) bool {
	a := Acquired(w, id)
	if a == nil || a.Status != "satisfied" || a.Value == nil || a.Value.Outcome != "done" {
		return false
	}
	raw := Result(w, a.Source.CommandID)
	return raw != nil && raw.Succeeded && nativeOutcomeAllowsAcquisition(raw.Outcome) && raw.FinalVerified && raw.Failure == "" && Source(*raw) == a.Source
}

// Prerequisites are frozen result references, not whichever turn most recently
// succeeded. Renewing control authorization cannot repair an invalid source.
func VerifyPrerequisites(w Snapshot, input StageInput, state State) error {
	if state.Contract == "publication" {
		if input.ImplementationResultID != w.ImplementationResultID || !trustedDone(w, input.ImplementationResultID) {
			return errors.New("publication implementation prerequisite is invalid")
		}
		if w.RequiresWorkspace() && (input.AuditResultID != w.AuditResultID || !AuditsPassed(w) || !trustedDone(w, input.AuditResultID)) {
			return errors.New("publication audit prerequisite is invalid")
		}
	}
	if state.Contract == "audit" {
		original := input
		if state.Kind == "report" {
			source := Input(w, input.SourceCommandID)
			if source == nil {
				return errors.New("audit report source missing")
			}
			original = *source
		}
		if !AuditSource(w, original) || !trustedDone(w, original.ImplementationResultID) {
			return errors.New("audit implementation prerequisite is invalid")
		}
	}
	return nil
}

func sameWorkspace(a, b *WorkspaceSource) bool {
	return a != nil && b != nil && a.Qualified && b.Qualified && !a.Dirty && !b.Dirty && a.Repository == b.Repository && a.Worktree == b.Worktree && a.Branch == b.Branch && a.Base == b.Base && a.Head == b.Head && a.Tree == b.Tree
}

// VerifyCompletion gates only done/normal successors. Incomplete business
// information is retained without inventing a clean completion checkpoint.
func VerifyCompletion(w Snapshot, state State, original StageInput, source StageResult, current StageInput) error {
	if state.Workspace == "" {
		return nil
	}
	if source.Workspace == nil || !source.Workspace.Qualified || source.Workspace.Dirty {
		return errors.New("original clean completion checkpoint unavailable")
	}
	if state.Kind == "report" {
		checked := Result(w, current.CommandID)
		if checked == nil || !sameWorkspace(source.Workspace, checked.Workspace) {
			return errors.New("report changed or could not verify original completion checkpoint")
		}
	}
	if state.Contract == "audit" {
		impl := Acquired(w, original.ImplementationResultID)
		if impl == nil || impl.ID != w.ImplementationResultID {
			return errors.New("audit implementation result changed")
		}
		pre := Result(w, impl.Source.CommandID)
		if pre == nil || pre.Workspace == nil || !pre.Workspace.Qualified {
			return errors.New("implementation checkpoint unavailable")
		}
	}
	if state.Contract == "publication" {
		audit := Acquired(w, original.AuditResultID)
		if audit == nil || audit.ID != w.AuditResultID {
			return errors.New("publication audit result changed")
		}
		post := Result(w, audit.Source.CommandID)
		if post == nil || !sameWorkspace(post.Workspace, source.Workspace) {
			return errors.New("publication differs from audited revision")
		}
	}
	return nil
}
