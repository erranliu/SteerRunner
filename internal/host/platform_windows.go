//go:build windows

package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"golang.org/x/sys/windows"
)

func normalizeCase(value string) string { return strings.ToLower(value) }

func currentSID() (string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

func pipeName(dir string) (string, error) {
	sid, err := currentSID()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(sid + "\x00" + dir))
	return `\\.\pipe\steerrunner-` + hex.EncodeToString(digest[:]), nil
}

func listenPipe(name string) (net.Listener, error) {
	sid, err := currentSID()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(name, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")",
		InputBufferSize:    65536, OutputBufferSize: 65536,
	})
}

func dialPipe(ctx context.Context, name string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, name)
}

type fileLock struct {
	file    *os.File
	overlap windows.Overlapped
}

func tryLock(path string, create bool) (*fileLock, bool, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, false, err
	}
	lock := &fileLock{file: file}
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lock.overlap)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		file.Close()
		return nil, true, nil
	}
	if err != nil {
		file.Close()
		return nil, false, err
	}
	return lock, false, nil
}

func (lock *fileLock) Close() error {
	err := windows.UnlockFileEx(windows.Handle(lock.file.Fd()), 0, 1, 0, &lock.overlap)
	closeErr := lock.file.Close()
	return errors.Join(err, closeErr)
}

func writerBusy(dir string) (bool, error) {
	lock, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if lock != nil {
		lock.Close()
	}
	return busy, err
}

func replaceFile(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	// Status polling may still have the previous discovery file open without
	// FILE_SHARE_DELETE. Retry only that transient Windows sharing collision;
	// keep publication atomic and never remove the old endpoint first.
	for attempt := 0; attempt < 100; attempt++ {
		err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if err != windows.ERROR_SHARING_VIOLATION && err != windows.ERROR_ACCESS_DENIED {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return err
}

func processFacts() (string, bool, error) {
	process := windows.CurrentProcess()
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &created, &exited, &kernel, &user); err != nil {
		return "", false, err
	}
	inJob, err := inAnyJob(process)
	return strconv.FormatUint(uint64(created.HighDateTime)<<32|uint64(created.LowDateTime), 10), !inJob, err
}

func inAnyJob(process windows.Handle) (bool, error) {
	var value uint32
	ok, _, callErr := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob").Call(uintptr(process), 0, uintptr(unsafe.Pointer(&value)))
	if ok == 0 {
		return false, callErr
	}
	return value != 0, nil
}

// Start suspended: no instruction can touch state until Job isolation is
// verified. This is adapted from SteerLine's pure platform owner, not its actor.
func startDetached(ctx context.Context, executable, dir, startupID string) error {
	log, err := os.OpenFile(filepath.Join(dir, "logs", "launch.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	intent, err := json.Marshal(bootstrapProof{StartupID: startupID})
	if err != nil {
		return err
	}
	if err := os.WriteFile(bootstrapPath(dir), intent, 0600); err != nil {
		return err
	}
	command := exec.Command(executable, "serve", "--state-dir", dir, "--startup-id", startupID)
	command.Stdout, command.Stderr = log, log
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_BREAKAWAY_FROM_JOB | windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	parentInJob, parentErr := inAnyJob(windows.CurrentProcess())
	if parentErr == nil && parentInJob {
		var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		queryErr := windows.QueryInformationJobObject(0, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil)
		fmt.Fprintf(log, "launcher job flags=0x%x query=%v\n", limits.BasicLimitInformation.LimitFlags, queryErr)
	}
	if err := command.Start(); err != nil {
		_ = os.Remove(bootstrapPath(dir))
		fmt.Fprintf(log, "detached creation rejected: %v\n", err)
		// A containing Job may prohibit BREAKAWAY even for an ordinary user.
		// The broker still suspends and verifies the exact child token/image;
		// it cannot adopt an elevated or restricted caller's permissions.
		if parentErr == nil && parentInJob && errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return startBrokered(ctx, executable, dir, startupID)
		}
		return contracts.Fail(4, "detach_unsupported", "Cannot start a service independent of the caller Job; start serve from an independent terminal.")
	}
	fail := func(err error) error {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = os.Remove(bootstrapPath(dir))
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(command.Process.Pid))
	if err != nil {
		return fail(err)
	}
	inJob, jobErr := inAnyJob(process)
	windows.CloseHandle(process)
	if jobErr != nil || inJob {
		fmt.Fprintf(log, "job isolation rejected: in_job=%t error=%v\n", inJob, jobErr)
		failure := fail(contracts.Fail(4, "detach_unsupported", "Service Job isolation could not be verified; no service instruction was executed."))
		if jobErr == nil {
			return startBrokered(ctx, executable, dir, startupID)
		}
		return failure
	}
	if err := resumeProcess(command.Process.Pid); err != nil {
		fmt.Fprintf(log, "initial thread resume failed: %v\n", err)
		return fail(err)
	}
	return command.Process.Release()
}

func resumeProcess(pid int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := false
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(pid) {
			continue
		}
		thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if openErr != nil {
			return openErr
		}
		previous, resumeErr := windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if resumeErr != nil {
			return resumeErr
		}
		resumed = resumed || previous > 0
	}
	if resumed && errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil
	}
	return fmt.Errorf("initial thread could not be resumed: %w", err)
}
