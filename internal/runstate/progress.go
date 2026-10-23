package runstate

import "github.com/erranliu/SteerRunner/internal/contracts"

// Progress has its own sequence; writing it never changes the business revision.
type Progress struct {
	View                    contracts.ProgressFileView `json:"view"`
	Workspace               string                     `json:"workspace"`
	Binding                 string                     `json:"binding"`
	Directories             map[string]string          `json:"directories"`
	Initialized             bool                       `json:"initialized"`
	InitializationAttempted bool                       `json:"initialization_attempted"`
	HostID                  string                     `json:"host_id,omitempty"`
	WorkspaceInstance       string                     `json:"workspace_instance,omitempty"`
}
