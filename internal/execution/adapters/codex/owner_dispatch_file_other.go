//go:build !windows

package codexworker

import (
	"context"
	"os"
)

func readOwnerDispatchFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
