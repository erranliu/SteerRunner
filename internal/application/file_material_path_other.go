//go:build !windows

package application

import "os"

func materialPathRedirected(path string) bool {
	info, err := os.Lstat(path)
	return err != nil || info.Mode()&os.ModeSymlink != 0
}
