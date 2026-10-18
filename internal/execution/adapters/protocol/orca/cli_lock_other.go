//go:build !windows

package orca

import "context"

// The observed shared-file rename failure is specific to Windows.
func lockCLIFile(ctx context.Context, _ string) (func(), error) {
	return func() {}, ctx.Err()
}
