//go:build !windows

package collect

import (
	"errors"
	"os"
	"os/user"
	"strings"

	"github.com/erranliu/SteerRunner/internal/usage"
)

func localUsageIdentity() (string, string, error) {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "", "", err
	}
	account, err := user.Current()
	if err != nil {
		return "", "", err
	}
	machine := strings.TrimSpace(string(data))
	if machine == "" || account.Uid == "" {
		return "", "", errors.New("IDENTITY_UNVERIFIED")
	}
	return usage.Digest("machine-id", machine), usage.Digest("local-account", account.Uid), nil
}
