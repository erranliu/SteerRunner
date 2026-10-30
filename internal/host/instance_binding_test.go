package host

import (
	"context"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"testing"
)

func TestSRExpectedInstanceFencesReplacement(t *testing.T) {
	dir, err := Normalize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	pipe, err := pipeName(dir)
	if err != nil {
		t.Fatal(err)
	}
	replacement := endpoint{Service: contracts.ServiceInfo{InstanceID: "B", StateDir: dir, Protocol: contracts.Protocol}, Pipe: pipe, Token: "B-token"}
	if err := writeEndpoint(dir, replacement); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"submit", "reply", "resume", "inspect", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"} {
		_, fault := Call(context.Background(), dir, Options{Command: command, ExpectedInstanceID: "A"}, nil)
		if fault == nil || fault.Detail.Code != "stale_instance" {
			t.Fatalf("%s reached replacement: %v", command, fault)
		}
	}
	// Even a client using B's discovery credentials cannot bypass the original A precondition.
	service := server{endpoint: replacement}
	response, stop := service.dispatch(wireRequest{Protocol: contracts.Protocol, InstanceID: "B", StateDir: dir, Token: "B-token", Options: Options{Command: "submit", ExpectedInstanceID: "A"}})
	if response.OK || response.Error == nil || response.Error.Code != "stale_instance" || stop {
		t.Fatalf("server did not fence original binding: %+v", response)
	}
	for _, command := range []string{"artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git"} {
		response, stop := service.dispatch(wireRequest{Protocol: contracts.Protocol, InstanceID: "B", StateDir: dir, Token: "B-token", Options: Options{Command: command, ExpectedInstanceID: "A"}})
		if response.OK || response.Error == nil || response.Error.Code != "stale_instance" || stop {
			t.Fatalf("evidence reached replacement: %s %+v", command, response)
		}
	}
}
