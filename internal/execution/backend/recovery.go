package backend

// RecoveryRequirements are host-projected resource needs, without a workflow
// cursor, mutable aggregate, or input authorization.
type RecoveryRequirements struct {
	Stopped              bool
	WorkspaceCheckpoints bool
}
