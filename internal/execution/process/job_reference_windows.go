//go:build windows

package process

import (
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"golang.org/x/sys/windows"
)

func DescribeOwnedJob(handle b.StopHandle) (OwnedJobReference, error) {
	p, ok := handle.(*childProcess)
	if !ok {
		return OwnedJobReference{}, errors.New("original managed Job unavailable")
	}
	owner, err := CurrentIdentity()
	if err != nil {
		return OwnedJobReference{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return OwnedJobReference{}, errors.New("original managed Job closed")
	}
	return OwnedJobReference{Owner: *owner, Worker: p.identity, Handle: uint64(p.job)}, nil
}

// AttachOwnedJob pins the original Job before stopping its owner. An attached
// handle must be terminated explicitly on Run stop; service shutdown only closes
// this duplicate so the original owner can continue serving the existing TUI.
func AttachOwnedJob(ref OwnedJobReference) (b.ProcessTreeHandle, error) {
	if ref.Owner.Kind != "os" || ref.Owner.PID <= 0 || ref.Owner.Created == "" || ref.Owner.Executable == "" || ref.Handle == 0 || ref.Worker.Kind != "os" || ref.Worker.PID <= 0 {
		return nil, errors.New("original managed Job reference incomplete")
	}
	owner, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(ref.Owner.PID))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(owner)
	identity, err := IdentityFromHandle(owner, ref.Owner.PID)
	if err != nil || identity != ref.Owner {
		return nil, errors.New("original managed Job owner changed")
	}
	status, err := windows.WaitForSingleObject(owner, 0)
	if err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		return nil, errors.New("original managed Job owner exited")
	}
	var duplicate windows.Handle
	if err = windows.DuplicateHandle(owner, windows.Handle(ref.Handle), windows.CurrentProcess(), &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, err
	}
	p := &childProcess{job: duplicate, identity: ref.Worker}
	member, err := p.ContainsProcess(uint32(ref.Worker.PID))
	if err != nil || !member || Probe(&ref.Worker).State != "matches" {
		p.Close()
		return nil, errors.New("original managed Job worker changed")
	}
	return p, nil
}
