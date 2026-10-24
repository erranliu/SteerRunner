//go:build windows

package host

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSREndpointPublicationWithDiscoveryReader(t *testing.T) {
	for _, release := range []bool{true, false} {
		name := "persistent-reader"
		if release {
			name = "transient-reader"
		}
		t.Run(name, func(t *testing.T) {
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
			before := endpoint{Service: contracts.ServiceInfo{InstanceID: "original", StateDir: dir, Protocol: contracts.Protocol}, Pipe: pipe, Token: "original-token"}
			if err := writeEndpoint(dir, before); err != nil {
				t.Fatal(err)
			}
			// A discovery read uses a Windows handle without FILE_SHARE_DELETE.
			// Hold that real handle to make the publication collision deterministic.
			reader, err := os.Open(endpointPath(dir))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			after := before
			after.Service.InstanceID, after.Token = "replacement", "replacement-token"
			result := make(chan error, 1)
			started := time.Now()
			go func() { result <- writeEndpoint(dir, after) }()
			select {
			case err := <-result:
				t.Fatalf("publication did not wait for transient discovery reader: %v", err)
			case <-time.After(40 * time.Millisecond):
			}
			if release {
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-result:
				if release && err != nil || !release && err == nil {
					t.Fatalf("release=%t publication=%v", release, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("publication retry was not bounded")
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("publication exceeded its finite retry budget")
			}
			current, err := readEndpoint(dir)
			expected := before.Service.InstanceID
			if release {
				expected = after.Service.InstanceID
			}
			if err != nil || current.Service.InstanceID != expected {
				t.Fatalf("atomic endpoint publication lost original/replacement: %+v %v", current.Service, err)
			}
		})
	}
}
