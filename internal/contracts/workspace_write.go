package contracts

const (
	WorkspaceAcquire = "workspace acquire"
	WorkspaceRelease = "workspace release"
)

type WorkspaceReleaseInput struct {
	ExpectedRevision uint64 `json:"expected_revision"`
	LeaseID          string `json:"lease_id"`
}

type WorkspaceWriteView struct {
	Owner      string `json:"owner"`
	LeaseID    string `json:"lease_id,omitempty"`
	CanAcquire bool   `json:"can_acquire"`
}
