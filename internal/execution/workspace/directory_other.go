//go:build !windows

package workspace

import "errors"

func DirectoryIdentity(string) (string, error) {
	return "", errors.New("content directory identity requires Windows")
}
