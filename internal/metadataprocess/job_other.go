//go:build !windows

package metadataprocess

import (
	"errors"
	"os/exec"
)

func Start(*exec.Cmd) (func(), error) {
	return nil, errors.New("metadata processes supported only on Windows")
}

func StartOwned(*exec.Cmd) (func() error, error) {
	return nil, errors.New("metadata processes supported only on Windows")
}
