//go:build windows

package collect

import (
	"errors"

	"github.com/erranliu/SteerRunner/internal/usage"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func localUsageIdentity() (string, string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", "", err
	}
	defer key.Close()
	machine, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", "", err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", "", err
	}
	account := user.User.Sid.String()
	if machine == "" || account == "" {
		return "", "", errors.New("IDENTITY_UNVERIFIED")
	}
	return usage.Digest("windows-machine", machine), usage.Digest("windows-account", account), nil
}
