package host

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func bindingServer(t *testing.T) (*server, contracts.ExpectedServiceBinding) {
	t.Helper()
	dir, err := Normalize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binding := contracts.ExpectedServiceBinding{Contract: contracts.ServiceBindingContract, Protocol: contracts.Protocol, InstanceID: "original", StateDir: dir, ExecutableSHA256: strings.Repeat("a", 64), Version: "fixed", ResourceDigest: strings.Repeat("b", 64)}
	s := &server{endpoint: endpoint{Service: contracts.ServiceInfo{Protocol: binding.Protocol, InstanceID: binding.InstanceID, StateDir: dir, ExecutableSHA256: binding.ExecutableSHA256, ResourceDigest: binding.ResourceDigest, Version: binding.Version, Status: "ready"}, Token: "original-token"}}
	return s, binding
}

func boundWire(s *server, binding *contracts.ExpectedServiceBinding, command string) wireRequest {
	return wireRequest{Protocol: contracts.Protocol, InstanceID: s.endpoint.Service.InstanceID, StateDir: s.endpoint.Service.StateDir, Token: s.endpoint.Token, Options: Options{Command: command, ExpectedBinding: binding}}
}

func TestSRFullBindingRejectsBeforeEffects(t *testing.T) {
	s, original := bindingServer(t)
	for _, command := range contracts.ServiceBindingCommands() {
		for _, field := range []string{"instance", "state-dir", "exe", "version", "resource", "protocol", "missing", "stopping", "not-ready"} {
			t.Run(command+"/"+field, func(t *testing.T) {
				b := original
				s.stopping = field == "stopping"
				s.endpoint.Service.Status = "ready"
				switch field {
				case "instance":
					b.InstanceID = "replacement"
				case "state-dir":
					b.StateDir += "-foreign"
				case "exe":
					b.ExecutableSHA256 = strings.Repeat("c", 64)
				case "version":
					b.Version = "other"
				case "resource":
					b.ResourceDigest = strings.Repeat("d", 64)
				case "protocol":
					b.Protocol = "foreign/v1"
				case "missing":
					b.ExecutableSHA256 = ""
				case "not-ready":
					s.endpoint.Service.Status = "starting"
				}
				// A nil engine/store deliberately makes any handler entry fatal.
				response, stop := s.dispatch(boundWire(s, &b, command))
				if response.OK || response.Error == nil || stop {
					t.Fatalf("reached handler: %+v", response)
				}
			})
		}
	}
	s.stopping, s.endpoint.Service.Status = false, "ready"
	response, stop := s.dispatch(boundWire(s, &original, "capabilities"))
	if !response.OK || stop {
		t.Fatal(response)
	}
}

func TestSRFullBindingStopAdmissionOrder(t *testing.T) {
	s, binding := bindingServer(t)
	store, err := storage.Open(binding.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s.store = store
	body, _ := json.Marshal(contracts.ServiceStopInput{ExpectedInstanceID: binding.InstanceID})
	wire := boundWire(s, nil, "service stop")
	wire.Request = &contracts.Request{Protocol: contracts.Protocol, RequestID: "original-stop", Input: body}
	started, done := make(chan struct{}), make(chan bool, 1)
	// Hold the same read gate held through an admitted handler. Stop must wait
	// until that handler's acceptance has finished before closing new admission.
	s.mutex.RLock()
	go func() {
		close(started)
		response, stop := s.dispatch(wire)
		done <- response.OK && stop
	}()
	<-started
	select {
	case <-done:
		s.mutex.RUnlock()
		t.Fatal("stop crossed an admitted action")
	case <-time.After(25 * time.Millisecond):
	}
	s.mutex.RUnlock()
	select {
	case accepted := <-done:
		if !accepted {
			t.Fatal("stop not accepted after gate release")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("admission lock deadlocked")
	}
	response, stop := s.dispatch(boundWire(s, &binding, "submit"))
	if response.OK || response.Error == nil || response.Error.Code != "invalid_state" || stop {
		t.Fatal("new submission crossed accepted stop", response)
	}
}
