//go:build windows

package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"unsafe"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"golang.org/x/sys/windows"
)

func resumeCLI(pid int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(pid) {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		return err
	}
	return fmt.Errorf("no initial CLI thread: %w", err)
}

func runJobHelper() int {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 90
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if os.Getenv("STEERRUNNER_JOB_MODE") == "allow" {
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return 91
	}
	command := exec.Command(os.Getenv("STEERRUNNER_JOB_HELPER"), "service", "start", "--state-dir", os.Getenv("STEERRUNNER_JOB_DIR"), "--json")
	var output, diagnostics bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostics
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED, HideWindow: true}
	if err := command.Start(); err != nil {
		return 92
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return 93
	}
	err = windows.AssignProcessToJobObject(job, process)
	windows.CloseHandle(process)
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return 94
	}
	if err := resumeCLI(command.Process.Pid); err != nil {
		command.Process.Kill()
		command.Wait()
		return 95
	}
	err = command.Wait()
	if failure, ok := err.(*exec.ExitError); ok {
		if failure.ExitCode() != 4 || os.Getenv("STEERRUNNER_JOB_MODE") != "deny" {
			fmt.Fprintln(os.Stderr, diagnostics.String())
			return 96
		}
	} else if err != nil {
		return 97
	}
	fmt.Fprint(os.Stdout, output.String())
	return 0 // closing the kill-on-close parent Job is the actual test boundary
}

func testParentJobs(t *testing.T) {
	for _, mode := range []string{"allow", "deny"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "runner")
			helper := exec.Command(os.Args[0], "-test.run=^$")
			helper.Env = append(os.Environ(), "STEERRUNNER_JOB_HELPER="+runnerEXE, "STEERRUNNER_JOB_MODE="+mode, "STEERRUNNER_JOB_DIR="+dir)
			data, err := helper.Output()
			if err != nil {
				t.Fatalf("job helper failed: %v", err)
			}
			var response contracts.Response
			if err := contracts.DecodeStrict(data, &response); err != nil {
				t.Fatalf("job response: %s", data)
			}
			if mode == "allow" {
				if !response.OK {
					t.Fatalf("allowed breakaway rejected: %+v", response)
				}
				info := serviceInfo(t, response)
				cleanupService(t, dir, info)
				status := serviceInfo(t, call(t, dir, []string{"service", "status"}, "", 0))
				if status.InstanceID != info.InstanceID || !status.Detached || status.Status != "ready" {
					t.Fatal("service lost when parent Job closed")
				}
			} else {
				if response.OK || response.Error == nil || response.Error.Code != "detach_unsupported" {
					t.Fatalf("blocked breakaway not explicit: %+v", response)
				}
				if _, err := os.Stat(filepath.Join(dir, "runner.db")); !os.IsNotExist(err) {
					t.Fatal("blocked child executed storage instructions")
				}
			}
		})
	}
}

func killOwnedService(info contracts.ServiceInfo) error {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, uint32(info.PID))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &created, &exited, &kernel, &user); err != nil {
		return err
	}
	identity := strconv.FormatUint(uint64(created.HighDateTime)<<32|uint64(created.LowDateTime), 10)
	if identity != info.ProcessCreated {
		return fmt.Errorf("owned process identity changed")
	}
	return windows.TerminateProcess(process, 1)
}

func waitOwnedService(info contracts.ServiceInfo) error {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(info.PID))
	if err == windows.ERROR_INVALID_PARAMETER {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &created, &exited, &kernel, &user); err != nil {
		return err
	}
	identity := strconv.FormatUint(uint64(created.HighDateTime)<<32|uint64(created.LowDateTime), 10)
	if identity != info.ProcessCreated {
		return nil
	}
	status, err := windows.WaitForSingleObject(process, 5000)
	if err != nil {
		return err
	}
	if status != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("original service has not exited")
	}
	return nil
}
