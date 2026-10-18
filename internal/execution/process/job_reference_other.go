//go:build !windows

package process

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func DescribeOwnedJob(b.StopHandle) (OwnedJobReference, error) {
	return OwnedJobReference{}, errors.New("managed Job references require Windows")
}
func AttachOwnedJob(OwnedJobReference) (b.ProcessTreeHandle, error) {
	return nil, errors.New("managed Job references require Windows")
}
