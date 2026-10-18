package process

import b "github.com/erranliu/SteerRunner/internal/execution/backend"

// OwnedJobReference identifies an existing Windows Job in its original owner.
// It is not a process lookup or a permission to adopt an arbitrary PID tree.
type OwnedJobReference struct {
	Owner  b.ProcessIdentity `json:"owner"`
	Worker b.ProcessIdentity `json:"worker"`
	Handle uint64            `json:"handle"`
}
