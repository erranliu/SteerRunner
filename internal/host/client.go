package host

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func Call(ctx context.Context, dir string, options Options, input *contracts.Request) (contracts.Response, *contracts.Fault) {
	id := ""
	if input != nil {
		id = input.RequestID
	}
	endpoint, err := readEndpoint(dir)
	if err != nil {
		return contracts.Response{}, fault(err, contracts.Fail(4, "service_unavailable", "Service discovery is unavailable; start the service explicitly."))
	}
	if options.ExpectedInstanceID != "" && options.ExpectedInstanceID != endpoint.Service.InstanceID {
		return contracts.Response{}, contracts.Fail(5, "stale_instance", "Original Runner instance changed; restart SteerLine explicitly.")
	}
	connection, err := dialPipe(ctx, endpoint.Pipe)
	if err != nil {
		return contracts.Response{}, contracts.Fail(4, "service_unavailable", "Cannot connect to this directory's service.")
	}
	defer connection.Close()
	deadline := time.Now().Add(TimeoutSeconds * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return contracts.Response{}, contracts.Fail(4, "service_unavailable", "Cannot establish a bounded service connection.")
	}
	wire := wireRequest{Protocol: contracts.Protocol, InstanceID: endpoint.Service.InstanceID, StateDir: dir, Token: endpoint.Token, Options: options, Request: input}
	unknown := func() (contracts.Response, *contracts.Fault) {
		if options.Command == "open-native-session" {
			return contracts.Response{}, contracts.Fail(6, "request_outcome_unknown", "Native viewer may have opened; do not replay this operation.")
		}
		if input != nil {
			return contracts.Response{}, contracts.Fail(6, "request_outcome_unknown", "The original request outcome is unknown; inspect or retry the same request_id and input.")
		}
		return contracts.Response{}, contracts.Fail(4, "service_unavailable", "The service query response is unavailable.")
	}
	encoder := json.NewEncoder(connection)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(wire); err != nil {
		return unknown()
	}
	line, err := bufio.NewReader(io.LimitReader(connection, maxWireBytes+1)).ReadBytes('\n')
	if err != nil || len(line) > maxWireBytes {
		return unknown()
	}
	var response contracts.Response
	if contracts.DecodeStrict(line, &response) != nil || response.Protocol != contracts.Protocol || response.OK && response.Error != nil || !response.OK && response.Error == nil {
		return unknown()
	}
	if id == "" && response.RequestID != nil || id != "" && (response.RequestID == nil || *response.RequestID != id) {
		return unknown()
	}
	return response, nil
}

func Status(ctx context.Context, dir string) (contracts.ServiceInfo, *contracts.Fault) {
	response, callErr := Call(ctx, dir, Options{Command: "service status"}, nil)
	if callErr == nil && response.OK {
		var info contracts.ServiceInfo
		data, _ := json.Marshal(response.Data)
		if contracts.DecodeStrict(data, &info) == nil {
			return info, nil
		}
	}
	if callErr != nil && callErr.Detail.Code == "directory_conflict" {
		return contracts.ServiceInfo{}, callErr
	}
	busy, err := writerBusy(dir)
	if err != nil {
		return contracts.ServiceInfo{}, fault(err, contracts.Fail(4, "service_unavailable", "Cannot verify service ownership."))
	}
	status := "not_running"
	if busy {
		status = "unavailable"
	} else if _, err := os.Stat(bootstrapPath(dir)); err == nil {
		status = "startup_unverified"
	}
	info := contracts.ServiceInfo{StateDir: dir, Status: status, Protocol: contracts.Protocol}
	if status == "not_running" {
		// Hold the writer lease while reading: no replacement can invalidate
		// this proof and create resources between the ownership check and read.
		lock, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), false)
		if os.IsNotExist(err) {
			return info, nil
		}
		if err != nil {
			return info, fault(err, contracts.Fail(4, "service_unavailable", "Cannot verify shutdown ownership."))
		}
		if busy {
			info.Status = "unavailable"
			return info, nil
		}
		if lock != nil {
			defer lock.Close()
		}
		info.LastShutdown, err = readShutdown(dir)
		if err != nil {
			return info, fault(err, contracts.Fail(4, "service_unavailable", "Cannot read shutdown evidence."))
		}
	}
	return info, nil
}

func Ensure(ctx context.Context, dir, executable string) (contracts.ServiceInfo, *contracts.Fault) {
	if info, err := Status(ctx, dir); err == nil && info.Status == "ready" {
		return selectedService(info, executable)
	}
	for _, name := range []string{"service", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			return contracts.ServiceInfo{}, contracts.Fail(4, "service_start_failed", "Cannot create the requested service directory.")
		}
	}
	var launch *fileLock
	for contextAlive(ctx) {
		lock, busy, err := tryLock(filepath.Join(dir, "service", "launch.lock"), true)
		if err != nil {
			return contracts.ServiceInfo{}, fault(err, contracts.Fail(4, "service_start_failed", "Cannot lock service startup."))
		}
		if !busy {
			launch = lock
			break
		}
		if info, err := Status(ctx, dir); err == nil && info.Status == "ready" {
			return selectedService(info, executable)
		}
		if !pause(ctx) {
			break
		}
	}
	if launch == nil {
		return contracts.ServiceInfo{}, contracts.Fail(4, "service_start_failed", "Another startup is unresolved; query service status.")
	}
	defer launch.Close()
	info, statusErr := Status(ctx, dir)
	if statusErr != nil {
		return contracts.ServiceInfo{}, statusErr
	}
	if info.Status == "ready" {
		return selectedService(info, executable)
	}
	if info.Status == "unavailable" {
		return contracts.ServiceInfo{}, contracts.Fail(5, "directory_conflict", "The original writer still owns this directory; do not replace it.")
	}
	if info.Status == "startup_unverified" {
		return contracts.ServiceInfo{}, contracts.Fail(4, "startup_unverified", "The original independent startup is unresolved; inspect bootstrap.json and owned processes before starting again.")
	}
	startupID := contracts.NewID()
	startupPath := filepath.Join(dir, "service", "startup-"+startupID+".json")
	defer os.Remove(startupPath)
	if err := startDetached(ctx, executable, dir, startupID); err != nil {
		return contracts.ServiceInfo{}, fault(err, contracts.Fail(4, "service_start_failed", "Cannot start the resident service."))
	}
	for contextAlive(ctx) {
		if info, err := Status(ctx, dir); err == nil && info.Status == "ready" {
			return selectedService(info, executable)
		}
		if data, err := os.ReadFile(startupPath); err == nil {
			var failure startupFailure
			if contracts.DecodeStrict(data, &failure) == nil && failure.StartupID == startupID {
				return contracts.ServiceInfo{}, &contracts.Fault{Detail: failure.Error, Exit: failure.Exit}
			}
		}
		if !pause(ctx) {
			break
		}
	}
	return contracts.ServiceInfo{}, contracts.Fail(4, "service_start_failed", "Service readiness was not confirmed; inspect launch.log and service status.")
}

func pause(ctx context.Context) bool {
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func removeOwnEndpoint(dir, instance string) {
	value, err := readEndpoint(dir)
	if err == nil && value.Service.InstanceID == instance {
		_ = os.Remove(endpointPath(dir))
	}
}
