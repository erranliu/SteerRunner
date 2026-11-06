package orcatransport

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestProgressHostLegacyRouting(t *testing.T) {
	key := make([]byte, 32)
	digest := sha256.Sum256(key)
	for _, tc := range []struct{ name, environment, store, want, failure string }{
		{"local-ignores-store", "", "invalid", "local", ""},
		{"paired-current-key", "paired-worker", "valid", "sha256:" + hex.EncodeToString(digest[:]), ""},
		{"paired-missing-store", "paired-worker", "missing", "", "fixed Orca environment authentication is unavailable"},
		{"paired-invalid-store", "paired-worker", "invalid", "", "fixed Orca environment authentication is invalid"},
		{"profile-before-pairing", "paired-worker", "invalid", "", "invalid terminal profile codec"},
		{"protocol-before-pairing", "paired-worker", "invalid", "", "unsupported Orca terminal lifecycle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("ORCA_USER_DATA_PATH", dir)
			if tc.store != "missing" {
				raw := []byte("!")
				if tc.store == "valid" {
					raw = mustAgentJSON(map[string]any{"environments": []any{map[string]any{"id": "paired-worker", "preferredEndpointId": "original", "endpoints": []any{map[string]any{"id": "original", "endpoint": "ws://127.0.0.1:1", "deviceToken": "fixture-only", "publicKeyB64": base64.StdEncoding.EncodeToString(key)}}}}})
				}
				if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			l, native, _ := frozenFixtureOn(t, tc.environment)
			setup := l.setup.Clone()
			setup.ProcessHost = nil
			if tc.name == "profile-before-pairing" {
				setup.ProfileData[0].CodecVersion = -1
			}
			if tc.name == "protocol-before-pairing" {
				setup.Snapshot.Ref.Protocol = "unknown"
			}
			before := setup.Clone()
			host, err := ProgressHost(setup)
			if host != tc.want {
				t.Fatalf("host %q, want %q", host, tc.want)
			}
			if tc.failure != "" {
				assertConstructorError(t, err, errors.New(tc.failure))
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, setup) || len(native.calls) != 0 {
				t.Fatal("host lookup changed setup or dispatched native operation")
			}
		})
	}
}

func TestProgressHostRejectsStructuredAccessBeforeReturningLocal(t *testing.T) {
	l, _, _ := structuredFixture(t)
	setup := l.setup.Clone()
	setup.ProcessHost = nil
	var profile AgentSessionProfile
	if err := json.Unmarshal(setup.ProfileData[0].Payload, &profile); err != nil {
		t.Fatal(err)
	}
	profile.AccessMode = "read_only"
	var err error
	setup.ProfileData[0], err = b.NewImplementationDataEnvelope(setup.ProfileData[0], mustAgentJSON(profile))
	if err != nil {
		t.Fatal(err)
	}
	host, err := ProgressHost(setup)
	assertConstructorError(t, err, errors.New("structured Orca requires its frozen native profile and original host"))
	if host != "" {
		t.Fatal("invalid local configuration returned a host", host)
	}
}
