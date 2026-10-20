package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestR2ArtifactDiscoveryAndFileLimits(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(contracts.ArtifactDirectory))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < contracts.MaxDiscoveryEntries; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("report-%03d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := DiscoverArtifacts(root)
	if err != nil || len(candidates) != contracts.MaxDiscoveryEntries {
		t.Fatal("exact discovery bound refused", len(candidates), err)
	}
	if err := os.WriteFile(filepath.Join(dir, "one-more"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = DiscoverArtifacts(root)
	var fault *contracts.Fault
	if !errors.As(err, &fault) || fault.Detail.Code != "artifact_limit_exceeded" {
		t.Fatal("discovery overflow silently truncated or wrong failure", err)
	}
	for _, size := range []int{0, contracts.MaxArtifactBytes, contracts.MaxArtifactBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			data := bytes.Repeat([]byte{0xff}, size)
			if err := os.WriteFile(filepath.Join(dir, "bounded"), data, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadArtifactSource(root, "bounded")
			if size <= contracts.MaxArtifactBytes {
				if err != nil || !bytes.Equal(got, data) {
					t.Fatal("valid binary file refused", err)
				}
			} else if !errors.As(err, &fault) || fault.Detail.Code != "artifact_limit_exceeded" {
				t.Fatal("file overflow accepted or wrong failure", err)
			}
		})
	}
}

func TestR2ArtifactChangesDuringReadNeverReturnBytes(t *testing.T) {
	for _, change := range []string{"grow", "replace", "read-error"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, filepath.FromSlash(contracts.ArtifactDirectory))
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "report")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			got, err := readArtifactSource(root, "report", func(reader io.Reader) ([]byte, error) {
				calls++
				data, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if change == "read-error" {
					return data, io.ErrUnexpectedEOF
				}
				if change == "replace" {
					if err := os.Rename(path, path+".old"); err != nil {
						t.Fatal(err)
					}
				}
				value := []byte("original plus new bytes")
				if change == "replace" {
					value = []byte("original")
				}
				if err := os.WriteFile(path, value, 0600); err != nil {
					t.Fatal(err)
				}
				return data, nil
			})
			var fault *contracts.Fault
			want := "artifact_changed"
			if change == "read-error" {
				want = "artifact_unavailable"
			}
			if got != nil || calls != 1 || !errors.As(err, &fault) || fault.Detail.Code != want {
				t.Fatal("changed/partial bytes were returned or wrong failure", len(got), calls, err)
			}
		})
	}
}
