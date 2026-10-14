//go:build windows

package host

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"golang.org/x/sys/windows"
)

// Win32_Process.Create creates outside the caller's Job chain. We use it only
// after native BREAKAWAY creation succeeded but retained a Job association.
// The child stays suspended until its executable and token have been checked.
// https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
const suspendedBroker = `
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
try {
  $request = [Console]::In.ReadToEnd() | ConvertFrom-Json
  $startup = New-CimInstance Win32_ProcessStartup -ClientOnly -Property @{ShowWindow=[uint16]0; CreateFlags=[uint32]524}
  $created = Invoke-CimMethod Win32_Process -MethodName Create -Arguments @{CommandLine=$request.command_line; CurrentDirectory=$request.directory; ProcessStartupInformation=$startup}
  @{code=[uint32]$created.ReturnValue; pid=[uint32]$created.ProcessId} | ConvertTo-Json -Compress
} catch { exit 1 }
`

type bootstrapProof struct {
	StartupID string `json:"startup_id"`
	PID       int    `json:"pid"`
	Created   string `json:"created"`
	SID       string `json:"sid"`
}

func bootstrapPath(dir string) string { return filepath.Join(dir, "service", "bootstrap.json") }
func proofPath(dir, id string) string { return filepath.Join(dir, "service", "proof-"+id+".json") }

func tokenIntegrity(token windows.Token) (string, error) {
	var size uint32
	_ = windows.GetTokenInformation(token, windows.TokenIntegrityLevel, nil, 0, &size)
	if size == 0 {
		return "", fmt.Errorf("token integrity unavailable")
	}
	buffer := make([]byte, size)
	if err := windows.GetTokenInformation(token, windows.TokenIntegrityLevel, &buffer[0], size, &size); err != nil {
		return "", err
	}
	label := (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buffer[0]))
	return label.Label.Sid.String(), nil
}

func tokenProfile(token windows.Token) (string, error) {
	groups, err := token.GetTokenGroups()
	if err != nil {
		return "", err
	}
	var profile []string
	for _, group := range groups.AllGroups() {
		// WMI can use a different logon session. Ignore its session SID;
		// the user SID and integrity are checked separately.
		if group.Attributes&windows.SE_GROUP_LOGON_ID == windows.SE_GROUP_LOGON_ID {
			continue
		}
		profile = append(profile, fmt.Sprintf("group:%s:%d", group.Sid.String(), group.Attributes))
	}
	var size uint32
	_ = windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &size)
	if size == 0 {
		return "", fmt.Errorf("token privileges unavailable")
	}
	buffer := make([]byte, size)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], size, &size); err != nil {
		return "", err
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	for _, privilege := range privileges.AllPrivileges() {
		profile = append(profile, fmt.Sprintf("privilege:%d:%d:%d", privilege.Luid.HighPart, privilege.Luid.LowPart, privilege.Attributes))
	}
	sort.Strings(profile)
	return strings.Join(profile, "\n"), nil
}

func startBrokered(ctx context.Context, executable, dir, startupID string) error {
	caller := windows.GetCurrentProcessToken()
	restricted, err := caller.IsRestricted()
	if err != nil {
		return err
	}
	if restricted || caller.IsElevated() {
		return contracts.Fail(4, "detach_unsupported", "Independent startup requires an ordinary unelevated user token; start serve from an independent terminal.")
	}
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	sid, err := currentSID()
	if err != nil {
		return err
	}
	intent, _ := json.Marshal(bootstrapProof{StartupID: startupID, SID: sid})
	if err := os.WriteFile(bootstrapPath(dir), intent, 0600); err != nil {
		return err
	}
	unknown := func() error {
		return contracts.Fail(4, "startup_unverified", "The original independent startup is unresolved; inspect bootstrap.json and owned processes before starting again.")
	}
	args := []string{executable, "serve", "--state-dir", dir, "--startup-id", startupID}
	for i := range args {
		args[i] = syscall.EscapeArg(args[i])
	}
	payload, _ := json.Marshal(map[string]string{"command_line": strings.Join(args, " "), "directory": dir})
	command := exec.CommandContext(ctx, filepath.Join(systemDir, "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", suspendedBroker)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	command.Stdin = bytes.NewReader(payload)
	output, err := command.Output()
	if err != nil {
		return unknown()
	}
	var result struct {
		Code uint32 `json:"code"`
		PID  uint32 `json:"pid"`
	}
	if contracts.DecodeStrict(output, &result) != nil {
		return unknown()
	}
	if result.Code != 0 {
		_ = os.Remove(bootstrapPath(dir))
		return contracts.Fail(4, "detach_unsupported", "The system process broker rejected independent startup.")
	}
	if result.PID == 0 {
		return unknown()
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, result.PID)
	if err != nil {
		return unknown()
	}
	defer windows.CloseHandle(process)
	// The returned suspended process is owned by this invocation, not a PID
	// discovered by scanning. Failures after this point terminate only that handle.
	verified := false
	defer func() {
		if !verified {
			_ = windows.TerminateProcess(process, 1)
			_, _ = windows.WaitForSingleObject(process, 5000)
			_ = os.Remove(proofPath(dir, startupID))
			_ = os.Remove(bootstrapPath(dir))
		}
	}()
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	callerIntegrity, err := tokenIntegrity(caller)
	if err != nil {
		return err
	}
	childIntegrity, err := tokenIntegrity(token)
	if err != nil {
		return err
	}
	restricted, err = token.IsRestricted()
	if err != nil {
		return err
	}
	if user.User.Sid.String() != sid || restricted || token.IsElevated() || childIntegrity != callerIntegrity {
		return contracts.Fail(4, "detach_unsupported", "Independent startup did not preserve the caller's user and permissions.")
	}
	callerProfile, err := tokenProfile(caller)
	if err != nil {
		return err
	}
	childProfile, err := tokenProfile(token)
	if err != nil {
		return err
	}
	if childProfile != callerProfile {
		return contracts.Fail(4, "detach_unsupported", "Independent startup changed the caller's authorization groups or privileges.")
	}
	image := make([]uint16, 32768)
	size := uint32(len(image))
	if err := windows.QueryFullProcessImageName(process, 0, &image[0], &size); err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(windows.UTF16ToString(image[:size])), filepath.Clean(executable)) {
		return contracts.Fail(4, "detach_unsupported", "Independent startup executable identity did not match.")
	}
	var created, exited, kernel, used windows.Filetime
	if err := windows.GetProcessTimes(process, &created, &exited, &kernel, &used); err != nil {
		return err
	}
	proof := bootstrapProof{StartupID: startupID, PID: int(result.PID), Created: strconv.FormatUint(uint64(created.HighDateTime)<<32|uint64(created.LowDateTime), 10), SID: sid}
	data, _ := json.Marshal(proof)
	if err := os.WriteFile(proofPath(dir, startupID), data, 0600); err != nil {
		return err
	}
	if err := resumeProcess(int(result.PID)); err != nil {
		return err
	}
	verified = true
	return nil
}

func launchFacts(dir, startupID string) (string, bool, error) {
	created, detached, err := processFacts()
	if err != nil || startupID == "" {
		return created, detached, err
	}
	data, readErr := os.ReadFile(proofPath(dir, startupID))
	if os.IsNotExist(readErr) {
		return created, detached, nil
	}
	if readErr != nil {
		return "", false, readErr
	}
	var proof bootstrapProof
	sid, err := currentSID()
	if err != nil {
		return "", false, err
	}
	if contracts.DecodeStrict(data, &proof) != nil || proof.StartupID != startupID || proof.PID != os.Getpid() || proof.Created != created || proof.SID != sid {
		return "", false, contracts.Fail(4, "detach_unsupported", "Independent startup proof did not match this process.")
	}
	return created, true, nil
}

func completeBootstrap(dir, startupID string) {
	if startupID == "" {
		return
	}
	data, err := os.ReadFile(bootstrapPath(dir))
	var intent bootstrapProof
	if err == nil && contracts.DecodeStrict(data, &intent) == nil && intent.StartupID == startupID {
		_ = os.Remove(bootstrapPath(dir))
		_ = os.Remove(proofPath(dir, startupID))
	}
}
