package reliable

import b "github.com/erranliu/SteerRunner/internal/execution/backend"

// RecordTurnStart binds an already verified start to its accepted command.
// Callers retain their existing identity and pending-command admission checks.
func RecordTurnStart(session b.SessionContext, turnID string) b.SessionContext {
	session.CurrentCommandID = session.PendingCommandID
	session.CurrentTurnID = turnID
	session.PendingCommandID = ""
	session.PendingTurnID = ""
	return session
}

// ApplySessionReceipt merges technical bindings and accepted-input facts without
// touching the caller's Run, public status, workspace record or persistence.
// On rejection it returns the original snapshot and start fact unchanged.
func ApplySessionReceipt(s b.LifecycleSnapshot, workspace *b.WorkspaceInspection, receipt b.LifecycleReceipt, started bool) (b.LifecycleSnapshot, bool, error) {
	binding := BindingSnapshot{ProjectIdentity: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, CurrentAttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigurationDigest: s.ConfigurationDigest, Target: s.Ref.Target, SessionID: s.Session.SessionID, SessionGeneration: s.Session.Generation, PendingCommandID: s.Session.PendingCommandID, PendingTurnID: s.Session.PendingTurnID, CurrentTurnID: s.Session.CurrentTurnID, ImplementationData: s.Implementation}
	if s.Ref.External.Kind != "" {
		identity := s.Ref.External
		binding.Process = &identity
	}
	if workspace != nil {
		f := workspace
		binding.Workspace = Workspace{InspectionScope: f.InspectionScope, Repository: f.Repository, Worktree: f.Worktree, Branch: f.Branch, Base: f.Base, Head: f.Head, Dirty: f.Dirty, VerifiedAt: f.CheckedAt}
	}
	updated, err := MergeReceipt(binding, receipt)
	if err != nil {
		return s, started, err
	}
	s.Implementation = updated.ImplementationData
	s.Ref.Target = updated.Target
	s.Session.Target = updated.Target
	s.Session.SessionID = updated.SessionID
	s.Session.Generation = updated.SessionGeneration
	if updated.Process != nil {
		s.Ref.External = *updated.Process
	}
	if receipt.Workspace != nil {
		f := receipt.Workspace
		s.Workspace = b.WorkspaceIntent{Path: f.Worktree, Branch: f.Branch, BaseCommit: f.Base}
		s.Ref.WorkspacePath = f.Worktree
		s.Ref.WorkspaceBranch = f.Branch
	}
	if receipt.State.TurnID != "" && receipt.State.InputAccepted {
		if s.Session.PendingCommandID != "" {
			if receipt.State.TurnStarted {
				s.Session = RecordTurnStart(s.Session, receipt.State.TurnID)
				started = true
			} else {
				s.Session.PendingTurnID = receipt.State.TurnID
				started = false
			}
		}
		s.PendingTurn = nil
	}
	return s, started, nil
}
