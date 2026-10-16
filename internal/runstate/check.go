package runstate

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// CheckRecord has no development workflow or Task delivery state. Operations
// use the same durable operation identities as ordinary execution.
type CheckRecord struct {
	AgentSettled     bool                       `json:"agent_settled"`
	View             contracts.EnvironmentCheck `json:"view"`
	Nonce            string                     `json:"nonce"`
	Allowed          bool                       `json:"allowed"`
	StartCommand     string                     `json:"start_command"`
	CommandStates    map[string]string          `json:"command_states"`
	Operations       []b.Operation              `json:"operations"`
	Workspace        b.WorkspaceIntent          `json:"workspace"`
	RepositoryPath   string                     `json:"repository_path"`
	RepositoryCommon string                     `json:"repository_common"`
	DirectoryOwned   bool                       `json:"directory_owned"`
	LinkedWriteArgs  []string                   `json:"linked_write_args"`
	NativeRoot       *b.ProcessIdentity         `json:"native_root,omitempty"`
	NativeOwner      *b.ProcessIdentity         `json:"native_owner,omitempty"`
	Endpoint         string                     `json:"endpoint,omitempty"`
	Managed          bool                       `json:"managed"`
}

func (r *CheckRecord) Clone() *CheckRecord {
	raw, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	var next CheckRecord
	if err = json.Unmarshal(raw, &next); err != nil {
		panic(err)
	}
	return &next
}
